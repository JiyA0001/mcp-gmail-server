package auth

import (
	"encoding/json"
	"fmt"
	"mcp-gmail-server/internal/db"
	"net/http"
)

func SaveGoogleCredentials(w http.ResponseWriter, r *http.Request) {

	// 1️⃣ Get auth_token from cookie
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		http.Error(w, "Unauthorized - No cookie", http.StatusUnauthorized)
		return
	}

	claims, err := ValidateToken(cookie.Value)
	if err != nil {
		http.Error(w, "Unauthorized - Invalid token", http.StatusUnauthorized)
		return
	}

	email := claims.Email

	// 2️⃣ Decode request body
	var req struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}

	// 3️⃣ Update using email (NOT id)
	_, err = db.DB.Exec(`
        UPDATE users
        SET google_client_id = ?, google_client_secret = ?
        WHERE email = ?
    `, req.ClientID, req.ClientSecret, email)

	if err != nil {
		http.Error(w, "Failed to save credentials", 500)
		return
	}

	fmt.Println("Saved credentials for:", email)

	w.Write([]byte("Credentials saved"))
}
