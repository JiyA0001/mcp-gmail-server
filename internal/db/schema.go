package db

import (
	"log"
)

func CreateTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			role VARCHAR(50) DEFAULT 'user',
			google_client_id TEXT,
			google_client_secret TEXT,
			access_token TEXT,
			refresh_token TEXT,
			expiry DATETIME,
			active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			key_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
	}

	for _, query := range queries {
		_, err := DB.Exec(query)
		if err != nil {
			log.Fatalf("Failed to create table: %v\nQuery: %s", err, query)
		}
	}

	log.Println("Tables checked/created successfully")
}
