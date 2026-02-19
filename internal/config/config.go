package config

import (
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Config struct {
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	Endpoint      oauth2.Endpoint
	JWTSecret     string
	AllowedOrigin string
}

func LoadConfig() *Config {
	if os.Getenv("RAILWAY_ENVIRONMENT") == "" {
		_ = godotenv.Load()
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// Fallback for dev only, or panic in prod
		jwtSecret = "very-secret-key-change-me"
	}

	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:3000"
	}

	return &Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes: []string{
			"https://www.googleapis.com/auth/gmail.readonly",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint:      google.Endpoint,
		JWTSecret:     jwtSecret,
		AllowedOrigin: allowedOrigin,
	}
}
