package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/pkg/response"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// UserContextKey is the key used to store user claims in the request context
	UserContextKey contextKey = "user_claims"
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	SecretKey     string
	TokenExpiry   time.Duration
	Issuer        string
}

// DefaultJWTConfig returns the default JWT configuration
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		SecretKey:   "ybtech-mini-exchange-secret-key-2026", // override via env JWT_SECRET
		TokenExpiry: 24 * time.Hour,
		Issuer:      "mini-exchange",
	}
}

// GenerateToken creates a new JWT token for a user
func GenerateToken(config JWTConfig, user *domain.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"iss":      config.Issuer,
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(config.TokenExpiry).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.SecretKey))
}

// ParseToken validates and parses a JWT token string
func ParseToken(config JWTConfig, tokenString string) (*domain.UserClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(config.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return &domain.UserClaims{
			UserID:   claims["user_id"].(string),
			Username: claims["username"].(string),
		}, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

// AuthMiddleware creates an HTTP middleware that validates JWT tokens.
// Protected routes will have user claims available in the request context.
func AuthMiddleware(config JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.JSON(w, http.StatusUnauthorized, response.APIResponse{
					Success: false,
					Error:   "missing Authorization header",
				})
				return
			}

			// Expected format: "Bearer <token>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				response.JSON(w, http.StatusUnauthorized, response.APIResponse{
					Success: false,
					Error:   "invalid Authorization header format, expected 'Bearer <token>'",
				})
				return
			}

			claims, err := ParseToken(config, parts[1])
			if err != nil {
				log.Printf("AUTH: invalid token: %v", err)
				response.JSON(w, http.StatusUnauthorized, response.APIResponse{
					Success: false,
					Error:   "invalid or expired token",
				})
				return
			}

			// Store claims in request context for downstream handlers
			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext extracts user claims from the request context
func GetUserFromContext(r *http.Request) *domain.UserClaims {
	if claims, ok := r.Context().Value(UserContextKey).(*domain.UserClaims); ok {
		return claims
	}
	return nil
}

// ValidateWSToken validates a JWT token from WebSocket query parameter.
// WebSocket connections can't send custom headers, so we accept the token
// as a query parameter: ws://localhost:8080/ws?token=<jwt>
func ValidateWSToken(config JWTConfig, tokenString string) (*domain.UserClaims, error) {
	return ParseToken(config, tokenString)
}
