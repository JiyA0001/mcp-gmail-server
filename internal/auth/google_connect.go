package auth

import (
	"encoding/json"
	"fmt"
	"mcp-gmail-server/internal/db"
	"net/http"
)

func SaveGoogleCredentials(w http.ResponseWriter, r *http.Request) {
	user, err := GetUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}

	_, err = db.DB.Exec(`
        UPDATE users
        SET google_client_id = ?, google_client_secret = ?
        WHERE id = ?
    `, req.ClientID, req.ClientSecret, user.ID)

	if err != nil {
		http.Error(w, "Failed to save credentials", 500)
		return
	}
	fmt.Println("err:", err)

	cookie, err := r.Cookie("user_email")
	if err != nil {
		fmt.Println("Cookie not found:", err)
	} else {
		fmt.Println("Cookie value:", cookie.Value)
	}

	w.Write([]byte("Credentials saved"))
}
