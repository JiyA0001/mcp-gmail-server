package auth

import (
	"encoding/json"
	"mcp-gmail-server/internal/db"
	"net/http"
)

func SaveGoogleCredentials(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r) // your existing auth middleware

	var req struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}

	_, err := db.DB.Exec(`
        UPDATE users
        SET google_client_id = ?, google_client_secret = ?
        WHERE id = ?
    `, req.ClientID, req.ClientSecret, user.ID)

	if err != nil {
		http.Error(w, "Failed to save credentials", 500)
		return
	}

	w.Write([]byte("Credentials saved"))
}
