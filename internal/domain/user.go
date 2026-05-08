package domain

import "time"

// User represents a registered user in the system
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // never exposed in JSON
	CreatedAt time.Time `json:"created_at"`
}

// UserClaims represents JWT token claims
type UserClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}
