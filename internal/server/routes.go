package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"mcp-gmail-server/internal/auth"
	"mcp-gmail-server/internal/config"
	"mcp-gmail-server/internal/gmail"
	"mcp-gmail-server/internal/llm"
	"mcp-gmail-server/internal/mcp"

	"golang.org/x/oauth2"
)

// var oauthToken *oauth2.Token

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

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

	mux.HandleFunc("/oauth/login", func(w http.ResponseWriter, r *http.Request) {
		url := gmail.GetAuthURL(oauthConfig)
		log.Println("OAuth URL:", url)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})

	// mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
	// 	code := r.URL.Query().Get("code")
	// 	token, err := gmail.ExchangeToken(oauthConfig, code)
	// 	if err != nil {
	// 		http.Error(w, err.Error(), http.StatusInternalServerError)
	// 		return
	// 	}

	// 	oauthToken = token
	// 	fmt.Fprintln(w, "OAuth successful! You can now fetch emails.")
	// })

	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")

		token, err := gmail.ExchangeToken(oauthConfig, code)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create client using this token
		client := oauthConfig.Client(context.Background(), token)

		// Fetch logged-in user's email
		resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
		if err != nil {
			http.Error(w, "Failed to get user info", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		var userInfo struct {
			Email string `json:"email"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
			http.Error(w, "Failed to decode user info", http.StatusInternalServerError)
			return
		}

		// Save to DB
		err = auth.SaveUser(userInfo.Email, token)
		if err != nil {
			http.Error(w, "Failed to save user", http.StatusInternalServerError)
			return
		}

		// Set session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "user_email",
			Value:    userInfo.Email,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, os.Getenv("FRONTEND_URL"), http.StatusTemporaryRedirect)
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
	mux.HandleFunc("/mcp/search", func(w http.ResponseWriter, r *http.Request) {

		enableCORS(w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// if oauthToken == nil {
		// 	http.Error(w, "Not authenticated with Gmail", http.StatusUnauthorized)
		// 	return
		// }

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

		token := &oauth2.Token{
			AccessToken:  user.AccessToken,
			RefreshToken: user.RefreshToken,
			Expiry:       user.Expiry,
		}

		intent := r.URL.Query().Get("intent")
		if intent == "" {
			http.Error(w, "Intent is required", http.StatusBadRequest)
			return
		}

		service, err := gmail.NewGmailService(token)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Create dynamic LLM (Claude / Groq / Gemini)
		llmClient, err := llm.NewLLM()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// 1️⃣ Build Gmail query
		gmailQuery, err := mcp.BuildGmailQuery(llmClient, intent)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// 2️⃣ Fetch emails
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

		// 3️⃣ Run extraction
		result, err := mcp.RunExtraction(llmClient, intent, emailTexts)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// Wrap protected routes with API key middleware
	// mux.Handle("/mcp/", auth.Middleware(protectedMux))

	// Finally register mux globally
	http.Handle("/", mux)
}
