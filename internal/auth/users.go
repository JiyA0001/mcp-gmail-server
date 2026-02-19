package auth

import (
	"database/sql"
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
	var clientID, clientSecret, accessToken, refreshToken sql.NullString
	var expiry sql.NullTime

	err := db.DB.QueryRow(`
        SELECT id, email, role,
               google_client_id, google_client_secret,
               access_token, refresh_token, expiry
        FROM users
        WHERE email = ?
    `, email).Scan(
		&user.ID,
		&user.Email,
		&user.Role,
		&clientID,
		&clientSecret,
		&accessToken,
		&refreshToken,
		&expiry,
	)

	if err != nil {
		return nil, err
	}

	if clientID.Valid {
		user.GoogleClientID = clientID.String
	}
	if clientSecret.Valid {
		user.GoogleClientSecret = clientSecret.String
	}
	if accessToken.Valid {
		user.AccessToken = accessToken.String
	}
	if refreshToken.Valid {
		user.RefreshToken = refreshToken.String
	}
	if expiry.Valid {
		user.Expiry = expiry.Time
	}

	return &user, nil
}
