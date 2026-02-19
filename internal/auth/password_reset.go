package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"mcp-gmail-server/internal/config"
	"mcp-gmail-server/internal/db"
	"mcp-gmail-server/internal/gmail"

	"golang.org/x/oauth2"
)

// GenerateResetToken creates a secure random token and stores it in the DB
func GenerateResetToken(email string) (string, error) {
	// check if user exists
	user, err := GetUserFromDB(email)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}

	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	// expiry 1 hour
	expiry := time.Now().Add(1 * time.Hour)

	_, err = db.DB.Exec(`
		INSERT INTO password_resets (user_id, token, expires_at)
		VALUES (?, ?, ?)
	`, user.ID, token, expiry)

	if err != nil {
		return "", err
	}

	return token, nil
}

// ValidateResetToken checks if token is valid and returns the user ID
func ValidateResetToken(token string) (int, error) {
	var userID int
	var expiresAt time.Time

	err := db.DB.QueryRow(`
		SELECT user_id, expires_at 
		FROM password_resets 
		WHERE token = ?
	`, token).Scan(&userID, &expiresAt)

	if err != nil {
		return 0, fmt.Errorf("invalid token")
	}

	if time.Now().After(expiresAt) {
		return 0, fmt.Errorf("token expired")
	}

	return userID, nil
}

// ResetPassword updates the user's password and invalidates the token
func ResetPassword(token, newPassword string) error {
	userID, err := ValidateResetToken(token)
	if err != nil {
		return err
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Start transaction
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}

	// Update password
	_, err = tx.Exec("UPDATE users SET password_hash = ? WHERE id = ?", hash, userID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Delete used token (and potentially all tokens for this user to be safe, or just this one)
	// Let's delete just this one for now, or all to prevent replay if we had multiple.
	// Safest is delete used token.
	_, err = tx.Exec("DELETE FROM password_resets WHERE token = ?", token)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// SendResetEmail sends the reset token via Gmail API using the System Account
func SendResetEmail(recipient, token string) {
	// 1. Load Config
	cfg := config.LoadConfig()

	// Construct link (assuming AllowedOrigin doesn't have trailing slash, or logic handles it)
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", cfg.AllowedOrigin, token)

	// 2. Log to console regardless (for dev/backup)
	log.Printf("---------------------------------------------------------")
	log.Printf("PASSWORD RESET REQUEST FOR: %s", recipient)
	log.Printf("Reset Link: %s", resetLink)
	log.Printf("Token: %s", token)
	log.Printf("---------------------------------------------------------")

	if cfg.SystemEmail == "" {
		log.Println("Note: SYSTEM_EMAIL not set, skipping Gmail API send.")
		return
	}

	// 3. Get System User Credentials
	user, err := GetUserFromDB(cfg.SystemEmail)
	if err != nil {
		log.Printf("Error: System user '%s' not found in DB: %v. Cannot send email.", cfg.SystemEmail, err)
		return
	}

	if user.AccessToken == "" || user.RefreshToken == "" {
		log.Printf("Error: System user '%s' has no valid tokens. Cannot send email.", cfg.SystemEmail)
		return
	}

	// 4. Build Service
	// Since we are in 'auth' package, we can use BuildOAuthConfig(user) which is in oauth_builder.go
	oauthConfig := BuildOAuthConfig(user)

	oauthToken := &oauth2.Token{
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		Expiry:       user.Expiry,
		TokenType:    "Bearer",
	}

	srv, err := gmail.NewGmailService(oauthConfig, oauthToken)
	if err != nil {
		log.Printf("Error creating Gmail service: %v", err)
		return
	}

	// 5. Send Email
	subject := "Password Reset Request"
	body := fmt.Sprintf("Hello,\n\nYou requested a password reset. Please click the link below to set a new password:\n\n%s\n\nOr verify this token manually:\n%s\n\nThis link expires in 1 hour.", resetLink, token)

	err = gmail.SendEmail(srv, recipient, subject, body)
	if err != nil {
		log.Printf("Error sending email via Gmail API: %v", err)
	} else {
		log.Printf("Reset email sent successfully to %s via %s", recipient, cfg.SystemEmail)
	}
}
