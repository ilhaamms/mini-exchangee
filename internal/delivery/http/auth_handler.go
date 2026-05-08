package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/ilhaamms/ybtech/internal/delivery/http/middleware"
	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/internal/repository"
	"github.com/ilhaamms/ybtech/pkg/response"
)

var userSeq int64

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	userRepo  *repository.UserRepository
	jwtConfig middleware.JWTConfig
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(userRepo *repository.UserRepository, jwtConfig middleware.JWTConfig) *AuthHandler {
	return &AuthHandler{
		userRepo:  userRepo,
		jwtConfig: jwtConfig,
	}
}

// RegisterRequest represents the request body for user registration
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest represents the request body for user login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Register handles POST /api/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.BadRequest(w, "method not allowed, use POST")
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body: "+err.Error())
		return
	}

	// Validate input
	if req.Username == "" {
		response.BadRequest(w, "username is required")
		return
	}
	if len(req.Username) < 3 {
		response.BadRequest(w, "username must be at least 3 characters")
		return
	}
	if req.Email == "" {
		response.BadRequest(w, "email is required")
		return
	}
	if req.Password == "" {
		response.BadRequest(w, "password is required")
		return
	}
	if len(req.Password) < 6 {
		response.BadRequest(w, "password must be at least 6 characters")
		return
	}

	// Hash password using SHA-256 (simple approach; production would use bcrypt)
	hashedPassword := hashPassword(req.Password)

	// Generate unique user ID
	userID := fmt.Sprintf("USR%010d", atomic.AddInt64(&userSeq, 1))

	user := &domain.User{
		ID:        userID,
		Username:  req.Username,
		Email:     req.Email,
		Password:  hashedPassword,
		CreatedAt: time.Now(),
	}

	if err := h.userRepo.Save(user); err != nil {
		response.JSON(w, http.StatusConflict, response.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Generate JWT token
	token, err := middleware.GenerateToken(h.jwtConfig, user)
	if err != nil {
		response.InternalError(w, "failed to generate token: "+err.Error())
		return
	}

	response.Created(w, "user registered successfully", map[string]interface{}{
		"user": map[string]interface{}{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"created_at": user.CreatedAt,
		},
		"token": token,
	})
}

// Login handles POST /api/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.BadRequest(w, "method not allowed, use POST")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body: "+err.Error())
		return
	}

	if req.Username == "" || req.Password == "" {
		response.BadRequest(w, "username and password are required")
		return
	}

	// Look up user
	user, err := h.userRepo.GetByUsername(req.Username)
	if err != nil {
		response.JSON(w, http.StatusUnauthorized, response.APIResponse{
			Success: false,
			Error:   "invalid username or password",
		})
		return
	}

	// Verify password
	if user.Password != hashPassword(req.Password) {
		response.JSON(w, http.StatusUnauthorized, response.APIResponse{
			Success: false,
			Error:   "invalid username or password",
		})
		return
	}

	// Generate JWT token
	token, err := middleware.GenerateToken(h.jwtConfig, user)
	if err != nil {
		response.InternalError(w, "failed to generate token: "+err.Error())
		return
	}

	response.Success(w, "login successful", map[string]interface{}{
		"user": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
		"token": token,
	})
}

// hashPassword creates a SHA-256 hash of the password
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}
