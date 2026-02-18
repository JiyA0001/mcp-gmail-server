package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"mcp-gmail-server/internal/db"
)

type User struct {
	ID                 int
	Email              string
	Role               string
	GoogleClientID     string
	GoogleClientSecret string
	AccessToken        string
	RefreshToken       string
	Expiry             time.Time
}

type ctxKey string

const userCtxKey ctxKey = "user"

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "Missing API key", http.StatusUnauthorized)
			return
		}

		rawKey := strings.TrimPrefix(auth, "Bearer ")

		hashBytes := sha256.Sum256([]byte(rawKey))
		hash := hex.EncodeToString(hashBytes[:])

		var user User
		var active bool

		err := db.DB.QueryRow(`
			SELECT u.id, u.email, u.role, u.active
			FROM api_keys k
			JOIN users u ON u.id = k.user_id
			WHERE k.key_hash=? AND k.active=TRUE
		`, hash).Scan(&user.ID, &user.Email, &user.Role, &active)

		if err != nil || !active {
			http.Error(w, "Invalid or revoked key", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUser(r *http.Request) (*User, error) {
	userVal := r.Context().Value("user")
	if userVal == nil {
		return nil, errors.New("user not in context")
	}

	user, ok := userVal.(User)
	if !ok {
		return nil, errors.New("invalid user type")
	}

	return &user, nil
}
