package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"technik-server/config"
	"technik-server/dto"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	JWTCookieName = "access_token"
)

type JWTClaims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a new JWT token valid for 24 hours
func GenerateJWT(userID, email, role, secret string) (string, error) {
	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "technik-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateJWT verifies the token string and returns parsed claims
func ValidateJWT(tokenStr, secret string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid or expired JWT token")
	}

	return claims, nil
}

// AuthMiddleware validates JWT from Cookie or Authorization header
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		// 1. Try reading from HTTP-only Cookie
		cookieToken, err := c.Cookie(JWTCookieName)
		if err == nil && cookieToken != "" {
			tokenStr = cookieToken
		}

		// 2. Fallback to Authorization Header
		if tokenStr == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					tokenStr = parts[1]
				}
			}
		}

		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, dto.APIError{
				Success: false,
				Error:   "Authentication required. Missing access token.",
			})
			c.Abort()
			return
		}

		claims, err := ValidateJWT(tokenStr, cfg.JWTSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.APIError{
				Success: false,
				Error:   fmt.Sprintf("Authentication failed: %v", err),
			})
			c.Abort()
			return
		}

		// Inject user context into Gin request context
		c.Set("userId", claims.UserID)
		c.Set("userEmail", claims.Email)
		c.Set("userRole", claims.Role)

		c.Next()
	}
}
