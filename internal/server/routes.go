package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"mcp-gmail-server/internal/auth"
	"mcp-gmail-server/internal/config"
	"mcp-gmail-server/internal/db"
	"mcp-gmail-server/internal/gmail"
	"mcp-gmail-server/internal/llm"
	"mcp-gmail-server/internal/mcp"

	"golang.org/x/oauth2"
)

// var oauthToken *oauth2.Token

// CORS is handled by middleware in main.go

func RegisterRoutes(cfg *config.Config) {

	mux := http.NewServeMux()

	oauthConfig := gmail.GetOAuthConfig(
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.RedirectURL,
	)

	// Determine Cookie Settings based on Origin (Prod/HTTPS vs Dev/HTTP)
	// If the frontend is HTTPS (e.g. Vercel), we MUST use Secure + SameSite=None for cross-origin cookies.
	useSecureCookie := strings.HasPrefix(cfg.AllowedOrigin, "https://")
	cookieSameSite := http.SameSiteLaxMode
	if useSecureCookie {
		cookieSameSite = http.SameSiteNoneMode
	}

	// -------------------------
	// PUBLIC ROUTES
	// -------------------------

	// Public route for OAuth login
	// Updated to support BYOK: If user is logged in, try to use their keys.
	mux.HandleFunc("/oauth/login", func(w http.ResponseWriter, r *http.Request) {

		// Default to env config
		finalConfig := oauthConfig

		// Check if user is logged in
		cookie, err := r.Cookie("auth_token")
		if err == nil {
			claims, err := auth.ValidateToken(cookie.Value)
			if err == nil {
				user, err := auth.GetUserFromDB(claims.Email)
				if err == nil && user.GoogleClientID != "" && user.GoogleClientSecret != "" {
					// User has custom keys, use them!
					finalConfig = auth.BuildOAuthConfig(user)
				}
			}
		}

		url := gmail.GetAuthURL(finalConfig)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})

	mux.HandleFunc("/auth/signup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if body.Email == "" || body.Password == "" {
			http.Error(w, "Email and password are required", http.StatusBadRequest)
			return
		}

		// Hash password
		hash, err := auth.HashPassword(body.Password)
		if err != nil {
			http.Error(w, "Error processing password", http.StatusInternalServerError)
			return
		}

		// Create user
		err = auth.CreateUser(body.Email, hash)
		if err != nil {
			// Check for duplicate entry (simple string check for MySQL)
			// A better way is checking mysql error code 1062, but this suffices for now
			log.Printf("Signup error: %v", err)
			http.Error(w, "User already exists or DB error", http.StatusConflict)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "User created successfully"})
	})

	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		// enableCORS handled by main

		if r.Method != http.MethodPost {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil || body.Email == "" || body.Password == "" {
			http.Error(w, "Invalid email or password", http.StatusBadRequest)
			return
		}

		// Get User
		user, err := auth.GetUserFromDB(body.Email)
		if err != nil {
			// Use generic error message for security
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Verify Password
		if !auth.CheckPasswordHash(body.Password, user.PasswordHash) {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Generate JWT
		tokenString, err := auth.GenerateToken(user.ID, user.Email)
		if err != nil {
			http.Error(w, "Token generation failed", 500)
			return
		}

		// Set Secure Cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    tokenString,
			Path:     "/",
			HttpOnly: true,
			Secure:   useSecureCookie,
			SameSite: cookieSameSite,
			// No Expires/MaxAge means Session Cookie (clears on browser close)
		})

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Login successful"})
	})

	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		// Clear Cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   useSecureCookie,
			SameSite: cookieSameSite,
			MaxAge:   -1, // Delete immediately
			Expires:  time.Now().Add(-1 * time.Hour),
		})
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Logged out"})
	})

	mux.HandleFunc("/auth/forgot-password", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
			http.Error(w, "Invalid email", http.StatusBadRequest)
			return
		}

		token, err := auth.GenerateResetToken(body.Email)
		if err != nil {
			// Don't reveal if user exists
			log.Printf("Forgot password error for %s: %v", body.Email, err)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "If this email is registered, you will receive a reset link."})
			return
		}

		// Send Email (Mock -> Gmail API)
		auth.SendResetEmail(body.Email, token)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "If this email is registered, you will receive a reset link."})
	})

	mux.HandleFunc("/auth/reset-password", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Token       string `json:"token"`
			NewPassword string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" || body.NewPassword == "" {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		err := auth.ResetPassword(body.Token, body.NewPassword)
		if err != nil {
			log.Printf("Reset password error: %v", err)
			http.Error(w, "Invalid or expired token", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Password reset successfully"})
	})

	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing code", 400)
			return
		}

		// 1️⃣ Identify the Current User / Config
		// Default to global config
		conf := oauthConfig
		var currentUser *auth.User

		cookie, err := r.Cookie("auth_token")
		if err == nil {
			claims, err := auth.ValidateToken(cookie.Value)
			if err == nil {
				currentUser, _ = auth.GetUserFromDB(claims.Email) // Ignore err, handled below
			}
		}

		// If we found a logged-in user with custom keys, use them!
		if currentUser != nil && currentUser.GoogleClientID != "" && currentUser.GoogleClientSecret != "" {
			conf = auth.BuildOAuthConfig(currentUser)
		}

		// 2️⃣ Exchange Code for Token
		token, err := conf.Exchange(context.Background(), code)
		if err != nil {
			log.Printf("Token exchange error: %v", err)
			http.Redirect(w, r, cfg.AllowedOrigin+"?error=token_exchange_failed", http.StatusTemporaryRedirect)
			return
		}

		// 3️⃣ Fetch User Info (Verify Identity)
		// Use manual request to ensure Authorization header is set correctly
		req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
		if err != nil {
			log.Printf("Failed to create userinfo request: %v", err)
			http.Error(w, "Failed to create request", 500)
			return
		}

		log.Printf("DEBUG: AccessToken (len=%d): %s...", len(token.AccessToken), token.AccessToken[:10])
		log.Printf("DEBUG: TokenType: %s", token.TokenType)
		log.Printf("DEBUG: RefreshToken: %s", token.RefreshToken)
		log.Printf("DEBUG: Expiry: %v", token.Expiry)

		if token.AccessToken == "" {
			log.Println("❌ OAuth Error: Access Token is empty after exchange!")
			http.Redirect(w, r, cfg.AllowedOrigin+"?error=empty_token", http.StatusTemporaryRedirect)
			return
		}

		req.Header.Set("Authorization", "Bearer "+token.AccessToken)

		userInfo, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("UserInfo request failed: %v", err)
			http.Redirect(w, r, cfg.AllowedOrigin+"?error=user_info_req_failed", http.StatusTemporaryRedirect)
			return
		}
		defer userInfo.Body.Close()

		// Read body for debugging
		bodyBytes, _ := io.ReadAll(userInfo.Body)
		log.Printf("Google User Info Response (Status: %d): %s", userInfo.StatusCode, string(bodyBytes))

		// Reset body for decoding
		userInfo.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var googleUser struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(userInfo.Body).Decode(&googleUser); err != nil {
			log.Printf("Failed to decode google user: %v", err)
			http.Error(w, "Failed to decode google user", 500)
			return
		}

		// 4️⃣ Update the User
		// Logic:
		// - If user was logged in (currentUser), update THEIR record.
		// - If not logged in (public oauth login?), update based on Google Email.

		targetEmail := googleUser.Email
		if currentUser != nil {
			targetEmail = currentUser.Email
		}

		if targetEmail == "" {
			log.Println("❌ OAuth Error: Could not determine user email (Check scopes/cookies)")
			http.Redirect(w, r, cfg.AllowedOrigin+"?error=email_missing", http.StatusTemporaryRedirect)
			return
		}

		_, err = db.DB.Exec(`
			INSERT INTO users (email, access_token, refresh_token, expiry)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE 
				access_token=VALUES(access_token),
				refresh_token=VALUES(refresh_token),
				expiry=VALUES(expiry)
		`,
			targetEmail,
			token.AccessToken,
			token.RefreshToken,
			token.Expiry,
		)

		if err != nil {
			http.Error(w, "DB update failed", 500)
			return
		}

		// 5️⃣ Refresh Session
		// Get User ID for the target email
		var userID int
		err = db.DB.QueryRow("SELECT id FROM users WHERE email = ?", targetEmail).Scan(&userID)
		if err != nil {
			http.Error(w, "User lookup failed", 500)
			return
		}

		// Generate JWT for the TARGET user
		tokenString, err := auth.GenerateToken(userID, targetEmail)
		if err != nil {
			http.Error(w, "Token generation failed", 500)
			return
		}

		// Set Secure Cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    tokenString,
			Path:     "/",
			HttpOnly: true,
			Secure:   useSecureCookie,
			SameSite: cookieSameSite,
			// No Expires means Session Cookie
		})

		http.Redirect(w, r, cfg.AllowedOrigin, http.StatusTemporaryRedirect)
	})

	http.HandleFunc("/privacy", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("This app accesses Gmail data only for the authenticated user and does not store or share any data."))
	})

	http.HandleFunc("/terms", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("This app accesses Gmail data only for the authenticated user and does not store or share any data."))
	})

	// -------------------------
	// PROTECTED ROUTES
	// -------------------------

	searchHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// 1️⃣ Get logged-in user from JWT cookie
		cookie, err := r.Cookie("auth_token")
		if err != nil {
			log.Printf("DEBUG: searchHandler: No auth_token cookie found: %v", err)
			http.Error(w, "Unauthorized: No cookie", http.StatusUnauthorized)
			return
		}

		// log.Printf("DEBUG: searchHandler: Cookie found: %s...", cookie.Value[:15])

		claims, err := auth.ValidateToken(cookie.Value)
		if err != nil {
			log.Printf("DEBUG: searchHandler: Token validation failed: %v", err)
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		log.Printf("DEBUG: searchHandler: Valid token for user: %s", claims.Email)

		user, err := auth.GetUserFromDB(claims.Email)
		if err != nil {
			log.Printf("DEBUG: searchHandler: User not found in DB for email %s: %v", claims.Email, err)
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// 2️⃣ Build OAuth config dynamically
		oauthConfig := auth.BuildOAuthConfig(user)

		token := &oauth2.Token{
			AccessToken:  user.AccessToken,
			RefreshToken: user.RefreshToken,
			Expiry:       user.Expiry,
		}

		// 3️⃣ Create Gmail service correctly
		service, err := gmail.NewGmailService(oauthConfig, token)
		if err != nil {
			http.Error(w, fmt.Sprintf("Gmail service error: %v", err), 500)
			return
		}

		intent := r.URL.Query().Get("intent")
		if intent == "" {
			http.Error(w, "Intent is required", http.StatusBadRequest)
			return
		}

		// 4️⃣ Create LLM client
		llmClient, err := llm.NewLLM()
		if err != nil {
			http.Error(w, fmt.Sprintf("LLM init error: %v", err), 500)
			return
		}

		// 5️⃣ Build Gmail query
		gmailQuery, limit, err := mcp.BuildGmailQuery(llmClient, intent)
		if err != nil {
			http.Error(w, fmt.Sprintf("Query builder error: %v", err), 500)
			return
		}

		// 6️⃣ Fetch emails (Concurrent & Full Body)
		emails, err := gmail.FetchEmails(service, gmailQuery, limit)
		if err != nil {
			http.Error(w, fmt.Sprintf("Fetch emails error: %v", err), 500)
			return
		}

		var emailTexts []string
		for _, e := range emails {
			// Use Body if available, fallback to Snippet
			content := e.Body
			if len(content) > 2000 {
				content = content[:2000] + "...(truncated)"
			}
			if content == "" {
				content = e.Snippet
			}

			emailTexts = append(emailTexts,
				fmt.Sprintf("From: %s\nSubject: %s\nDate: %s\nContent: %s",
					e.From, e.Subject, e.Date, content),
			)
		}

		// 7️⃣ Run extraction
		result, err := mcp.RunExtraction(llmClient, intent, emailTexts)
		if err != nil {
			http.Error(w, fmt.Sprintf("Extraction error: %v", err), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// Register with Middleware?
	// The auth logic is now inside the handler via JWT check.
	// auth.Middleware checked API keys. We can strip it or keep it for other routes.
	// Here we use searchHandler directly.
	mux.Handle("/mcp/search", searchHandler)

	mux.HandleFunc("/auth/status", func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("auth_token")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]bool{"logged_in": false})
			return
		}

		claims, err := auth.ValidateToken(cookie.Value)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]bool{"logged_in": false})
			return
		}

		var googleClientID sql.NullString
		err = db.DB.QueryRow(`
			SELECT google_client_id
			FROM users
			WHERE email = ?
		`, claims.Email).Scan(&googleClientID)

		// Note: The google_client_id check might be legacy or for a different flow.
		// Since we use OAauth via /auth/callback, we know they are connected if they have a token.
		// But let's keep the query valid.

		// If user exists, they are logged in.
		// Check if they have access token
		var hasToken bool
		err = db.DB.QueryRow("SELECT IF(access_token IS NOT NULL AND access_token != '', TRUE, FALSE) FROM users WHERE email = ?", claims.Email).Scan(&hasToken)

		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{
			"logged_in":       true,
			"gmail_connected": hasToken,
			"has_credentials": googleClientID.Valid && googleClientID.String != "",
		})
	})

	mux.Handle("/connect/google", http.HandlerFunc(auth.SaveGoogleCredentials))

	// Finally register mux globally
	http.Handle("/", mux)
}
