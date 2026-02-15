package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func LoadConfig() *Config {
	if os.Getenv("RAILWAY_ENVIRONMENT") == "" {
		_ = godotenv.Load()
	}

	return &Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
	}
}
