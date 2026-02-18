package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"mcp-gmail-server/internal/auth"
	"mcp-gmail-server/internal/db"
)

func CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	// 1. Get current user (from auth middleware)
	adminUser, err := auth.GetUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Allow only admin
	if adminUser.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 3. Get user_id from query or body
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	// 4. Generate API key
	rawKey, hash, err := auth.GenerateAPIKey()
	if err != nil {
		http.Error(w, "failed to generate key", 500)
		return
	}

	// 5. Store ONLY hash in DB
	_, err = db.DB.Exec(`
		INSERT INTO api_keys (user_id, key_hash)
		VALUES (?, ?)
	`, userID, hash)

	if err != nil {
		http.Error(w, "failed to save key", 500)
		return
	}

	// 6. Return RAW key ONCE
	json.NewEncoder(w).Encode(map[string]string{
		"api_key": rawKey,
	})
}
