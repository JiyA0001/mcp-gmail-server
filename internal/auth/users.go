package auth

import (
	// "database/sql"

	"mcp-gmail-server/internal/db"

	"golang.org/x/oauth2"
)

func SaveUser(email string, token *oauth2.Token) error {
	_, err := db.DB.Exec(`
        INSERT INTO users (email, access_token, refresh_token, expiry)
        VALUES (?, ?, ?, ?)
        ON DUPLICATE KEY UPDATE
            access_token = VALUES(access_token),
            refresh_token = VALUES(refresh_token),
            expiry = VALUES(expiry)
    `,
		email,
		token.AccessToken,
		token.RefreshToken,
		token.Expiry,
	)
	return err
}

func GetUserFromDB(email string) (*User, error) {
	var user User

	err := db.DB.QueryRow(`
		SELECT id, email, role, access_token, refresh_token, expiry
		FROM users
		WHERE email = ?
	`, email).Scan(
		&user.ID,
		&user.Email,
		&user.Role,
		&user.AccessToken,
		&user.RefreshToken,
		&user.Expiry,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}
