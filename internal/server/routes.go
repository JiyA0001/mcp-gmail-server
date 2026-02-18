package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"mcp-gmail-server/internal/auth"
	"mcp-gmail-server/internal/config"
	"mcp-gmail-server/internal/db"
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

	// oauthConfig := gmail.GetOAuthConfig(
	// 	cfg.ClientID,
	// 	cfg.ClientSecret,
	// 	cfg.RedirectURL,
	// )

	// -------------------------
	// PUBLIC ROUTES
	// -------------------------

	// mux.HandleFunc("/oauth/login", func(w http.ResponseWriter, r *http.Request) {
	// 	url := gmail.GetAuthURL(oauthConfig)
	// 	log.Println("OAuth URL:", url)
	// 	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	// })

	mux.HandleFunc("/oauth/login", func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUser(r)

		dbUser, err := auth.GetUserFromDB(user.Email)
		if err != nil {
			http.Error(w, "User not found", 401)
			return
		}

		oauthConfig := auth.BuildOAuthConfig(dbUser)

		url := oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})

	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUser(r)

		dbUser, err := auth.GetUserFromDB(user.Email)
		if err != nil {
			http.Error(w, "User not found", 401)
			return
		}

		oauthConfig := auth.BuildOAuthConfig(dbUser)

		code := r.URL.Query().Get("code")

		token, err := oauthConfig.Exchange(r.Context(), code)
		if err != nil {
			http.Error(w, "Token exchange failed", 500)
			return
		}

		_, err = db.DB.Exec(`
			UPDATE users
			SET access_token = ?, refresh_token = ?, expiry = ?
			WHERE id = ?
		`,
			token.AccessToken,
			token.RefreshToken,
			token.Expiry,
			dbUser.ID,
		)

		if err != nil {
			http.Error(w, "Failed to save token", 500)
			return
		}

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
		service, err := gmail.NewGmailService(r.Context(), oauthConfig, token)
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

	mux.HandleFunc("/connect/google", auth.SaveGoogleCredentials)

	// Wrap protected routes with API key middleware
	mux.Handle("/mcp/", auth.Middleware(mux))

	// Finally register mux globally
	http.Handle("/", mux)
}
