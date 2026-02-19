package db

import (
	"log"
	"strings"
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
			password_hash VARCHAR(255),
			active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		// Try to add column for existing tables (syntax compatible with older MySQL)
		// We ignore "Duplicate column" error below
		`ALTER TABLE users ADD COLUMN password_hash VARCHAR(255);`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			key_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS password_resets (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			token VARCHAR(255) NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
	}

	for _, query := range queries {
		_, err := DB.Exec(query)
		if err != nil {
			// Ignore "Duplicate column name" error (MySQL Error 1060) which happens if column exists
			// This is necessary because older MySQL versions don't support "ADD COLUMN IF NOT EXISTS"
			if strings.Contains(err.Error(), "Duplicate column name") {
				continue
			}

			log.Fatalf("Failed to create table/column: %v\nQuery: %s", err, query)
		}
	}

	log.Println("Tables checked/created successfully")
}
