package domain

import "time"

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` 
	CreatedAt time.Time `json:"created_at"`
}

type UserClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}
