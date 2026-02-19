package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"mcp-gmail-server/internal/auth"
	"mcp-gmail-server/internal/config"
	"mcp-gmail-server/internal/db"
	"mcp-gmail-server/internal/server"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// List of allowed origins
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"https://mcp-gmail-frontend.vercel.app",
		}

		origin := r.Header.Get("Origin")

		// If Origin is missing, try to determine it from Referer
		if origin == "" {
			referer := r.Header.Get("Referer")
			if referer != "" {
				for _, allowed := range allowedOrigins {
					// Check if referer is exactly the allowed origin or starts with allowed origin + "/"
					if referer == allowed || strings.HasPrefix(referer, allowed+"/") {
						origin = allowed
						break
					}
				}
			}
		}

		// Check if origin is allowed
		allowed := false
		for _, o := range allowedOrigins {
			if o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	cfg := config.LoadConfig()

	// Initialize JWT
	auth.InitJWT(cfg.JWTSecret)

	server.RegisterRoutes(cfg)
	db.Init()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Server running on port:", port)
	log.Println("Allowed Origin:", cfg.AllowedOrigin)

	// Wrap specific mux instead of DefaultServeMux if possible, but here logic uses DefaultServeMux inside RegisterRoutes (no, it uses mux inside RegisterRoutes but registers to DefaultServeMux at the end).
	// Actually RegisterRoutes does: http.Handle("/", mux)
	// So we should wrap http.DefaultServeMux

	log.Fatal(http.ListenAndServe(":"+port, corsMiddleware(http.DefaultServeMux)))
}
