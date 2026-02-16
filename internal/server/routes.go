package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"mcp-gmail-server/internal/auth"
	"mcp-gmail-server/internal/config"
	"mcp-gmail-server/internal/gmail"
	"mcp-gmail-server/internal/llm"
	"mcp-gmail-server/internal/mcp"

	"golang.org/x/oauth2"
)

var oauthToken *oauth2.Token

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
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})

	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		token, err := gmail.ExchangeToken(oauthConfig, code)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		oauthToken = token
		fmt.Fprintln(w, "OAuth successful! You can now fetch emails.")
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

	protectedMux := http.NewServeMux()

	protectedMux.HandleFunc("/mcp/search", func(w http.ResponseWriter, r *http.Request) {

		enableCORS(w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if oauthToken == nil {
			http.Error(w, "Not authenticated with Gmail", http.StatusUnauthorized)
			return
		}

		intent := r.URL.Query().Get("intent")
		if intent == "" {
			http.Error(w, "Intent is required", http.StatusBadRequest)
			return
		}

		service, err := gmail.NewGmailService(oauthToken)
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
	mux.Handle("/mcp/", auth.Middleware(protectedMux))

	// Finally register mux globally
	http.Handle("/", mux)
}
