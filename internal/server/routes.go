package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

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

	// -------------------------
	// PUBLIC ROUTES
	// -------------------------

	// mux.HandleFunc("/oauth/login", func(w http.ResponseWriter, r *http.Request) {
	// 	url := gmail.GetAuthURL(oauthConfig)
	// 	log.Println("OAuth URL:", url)
	// 	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	// })

	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		// enableCORS handled by main

		if r.Method != http.MethodPost {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Email string `json:"email"`
		}

		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil || body.Email == "" {
			http.Error(w, "Invalid email", http.StatusBadRequest)
			return
		}

		// Create user if not exists
		_, err = db.DB.Exec(`
			INSERT INTO users (email)
			VALUES (?)
			ON DUPLICATE KEY UPDATE email=email
		`, body.Email)

		if err != nil {
			http.Error(w, "DB error", 500)
			return
		}

		// Set cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "user_email",
			Value:    body.Email,
			Path:     "/",
			HttpOnly: true,
		})

		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing code", 400)
			return
		}

		// 1. Exchange Access Token
		token, err := oauthConfig.Exchange(context.Background(), code)
		if err != nil {
			http.Error(w, "Token exchange failed", 500)
			return
		}

		// 2. Fetch User Info (Verify Identity)
		client := oauthConfig.Client(context.Background(), token)
		userInfo, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
		if err != nil {
			http.Error(w, "Failed to get user info from Google", 500)
			return
		}
		defer userInfo.Body.Close()

		var googleUser struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(userInfo.Body).Decode(&googleUser); err != nil {
			http.Error(w, "Failed to decode google user", 500)
			return
		}

		// Update or Create the user securely based on Google's verified email
		_, err = db.DB.Exec(`
			INSERT INTO users (email, access_token, refresh_token, expiry)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE 
				access_token=VALUES(access_token),
				refresh_token=VALUES(refresh_token),
				expiry=VALUES(expiry)
		`,
			googleUser.Email,
			token.AccessToken,
			token.RefreshToken,
			token.Expiry,
		)

		if err != nil {
			http.Error(w, "DB update failed", 500)
			return
		}

		http.Redirect(w, r, "http://localhost:3000", http.StatusTemporaryRedirect)
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

	// protectedMux := http.NewServeMux()

	// protectedMux.HandleFunc("/mcp/search", func(w http.ResponseWriter, r *http.Request) {
	// 🔹 PROTECTED ROUTES (Applied strictly to specific handlers)
	// We wrap the sensitive handlers with auth.Middleware explicitly

	searchHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// 1️⃣ Get logged-in user from cookie
		cookie, err := r.Cookie("user_email")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := auth.GetUserFromDB(cookie.Value)
		if err != nil {
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
			http.Error(w, err.Error(), 500)
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
			http.Error(w, err.Error(), 500)
			return
		}

		// 5️⃣ Build Gmail query
		gmailQuery, err := mcp.BuildGmailQuery(llmClient, intent)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// 6️⃣ Fetch emails
		emails, err := gmail.FetchEmailsPaged(service, gmailQuery, 1)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		var emailTexts []string
		for _, e := range emails {
			emailTexts = append(emailTexts,
				fmt.Sprintf("From: %s\nSubject: %s\nBody: %s",
					e.From, e.Subject, e.Snippet),
			)
		}

		// 7️⃣ Run extraction
		result, err := mcp.RunExtraction(llmClient, intent, emailTexts)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// Register with Middleware
	mux.Handle("/mcp/search", auth.Middleware(searchHandler))

	mux.HandleFunc("/auth/status", func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("user_email")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]bool{
				"logged_in": false,
			})
			return
		}

		var googleClientID sql.NullString

		err = db.DB.QueryRow(`
			SELECT google_client_id
			FROM users
			WHERE email = ?
		`, cookie.Value).Scan(&googleClientID)

		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]bool{
				"logged_in": false,
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{
			"logged_in":       true,
			"gmail_connected": googleClientID.Valid && googleClientID.String != "",
		})
	})

	// mux.Handle("/connect/google",
	// 	auth.Middleware(http.HandlerFunc(auth.SaveGoogleCredentials)),
	// )
	mux.Handle("/connect/google", http.HandlerFunc(auth.SaveGoogleCredentials))

	// Finally register mux globally
	http.Handle("/", mux)
}
