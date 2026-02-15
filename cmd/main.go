package main

import (
	"log"
	"net/http"
	"os"

	"mcp-gmail-server/internal/config"
	"mcp-gmail-server/internal/db"
	"mcp-gmail-server/internal/server"
)

func main() {
	cfg := config.LoadConfig()
	server.RegisterRoutes(cfg)
	if err := db.Init(); err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Server running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
