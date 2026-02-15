package admin

import (
	"encoding/json"
	"net/http"

	"mcp-gmail-server/internal/auth"
	"mcp-gmail-server/internal/db"
)

func ListUsers(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	rows, _ := db.DB.Query(`
		SELECT id, email, role, active, created_at FROM users
	`)

	defer rows.Close()

	var result []map[string]interface{}

	for rows.Next() {
		var id int
		var email, role string
		var active bool
		var created string

		rows.Scan(&id, &email, &role, &active, &created)

		result = append(result, map[string]interface{}{
			"id":         id,
			"email":      email,
			"role":       role,
			"active":     active,
			"created_at": created,
		})
	}

	json.NewEncoder(w).Encode(result)
}
