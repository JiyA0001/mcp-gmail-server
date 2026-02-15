package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	oauthConfig := gmail.GetOAuthConfig(
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.RedirectURL,
	)

	http.HandleFunc("/oauth/login", func(w http.ResponseWriter, r *http.Request) {
		url := gmail.GetAuthURL(oauthConfig)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})

	http.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		token, err := gmail.ExchangeToken(oauthConfig, code)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		oauthToken = token
		fmt.Fprintln(w, "OAuth successful! You can now fetch emails.")
	})

	http.HandleFunc("/emails", func(w http.ResponseWriter, r *http.Request) {
		if oauthToken == nil {
			http.Error(w, "Not authenticated. Login first.", http.StatusUnauthorized)
			return
		}

		query := r.URL.Query().Get("q")
		if strings.TrimSpace(query) == "" {
			query = ""
		}

		service, err := gmail.NewGmailService(oauthToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		emails, err := gmail.FetchEmails(service, query, 10)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emails)
	})

	protected := auth.Middleware(mux)
	protected.HandleFunc("/mcp/search", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if oauthToken == nil {
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}

		intent := r.URL.Query().Get("intent")
		if intent == "" {
			http.Error(w, "Intent is required", http.StatusBadRequest)
			return
		}

		service, _ := gmail.NewGmailService(oauthToken)
		// gemini := llm.NewGeminiClient(os.Getenv("GEMINI_API_KEY"))
		// groq := llm.NewGroqClient(os.Getenv("GROQ_API_KEY"))
		llmClient, err := llm.NewLLM()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// 1️⃣ Build Gmail query using LLM
		gmailQuery, err := mcp.BuildGmailQuery(llmClient, intent)
		// gmailQuery, err := mcp.BuildGmailQuery(groq, intent)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// 2️⃣ Fetch ALL relevant emails
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

		// gemini = llm.NewGeminiClient(os.Getenv("GEMINI_API_KEY"))
		// llmClient, err := llm.NewLLM()
		// if err != nil {
		// 	http.Error(w, err.Error(), 500)
		// 	return
		// }
		// groq = llm.NewGroqClient(os.Getenv("GROQ_API_KEY"))

		result, err := mcp.RunExtraction(llmClient, intent, emailTexts)
		// result, err := mcp.RunExtraction(groq, intent, emailTexts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(result)
	})

}
