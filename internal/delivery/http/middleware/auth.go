package middleware

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ilhaamms/ybtech/internal/domain"
	"github.com/ilhaamms/ybtech/pkg/response"
)

type contextKey string

const (
	UserContextKey contextKey = "user_claims"
)

type JWTConfig struct {
	SecretKey   string
	TokenExpiry time.Duration
	Issuer      string
}

func DefaultJWTConfig() JWTConfig {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "ybtech-mini-exchange-secret-key-2026"
		log.Println("WARNING: JWT_SECRET env not set, using default (insecure for production)")
	}
	return JWTConfig{
		SecretKey:   secret,
		TokenExpiry: 24 * time.Hour,
		Issuer:      "mini-exchange",
	}
}

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
		userID, ok1 := claims["user_id"].(string)
		username, ok2 := claims["username"].(string)
		if !ok1 || !ok2 {
			return nil, jwt.ErrSignatureInvalid
		}
		return &domain.UserClaims{
			UserID:   userID,
			Username: username,
		}, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

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

			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserFromContext(r *http.Request) *domain.UserClaims {
	if claims, ok := r.Context().Value(UserContextKey).(*domain.UserClaims); ok {
		return claims
	}
	return nil
}

func ValidateWSToken(config JWTConfig, tokenString string) (*domain.UserClaims, error) {
	return ParseToken(config, tokenString)
}
