package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"technik-server/config"
	"technik-server/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	CSRFCookieName = "csrf_token"
	CSRFHeaderName = "X-CSRF-Token"
)

// GenerateCSRFToken creates a cryptographically signed CSRF token
func GenerateCSRFToken(secret string) string {
	rawID := uuid.New().String()
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	data := rawID + ":" + timestamp

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	signature := hex.EncodeToString(mac.Sum(nil))

	return data + ":" + signature
}

// ValidateCSRFToken checks if the token signature is valid
func ValidateCSRFToken(tokenStr, secret string) bool {
	parts := strings.Split(tokenStr, ":")
	if len(parts) != 3 {
		return false
	}
	rawID := parts[0]
	timestamp := parts[1]
	providedSig := parts[2]

	data := rawID + ":" + timestamp
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(providedSig), []byte(expectedSig))
}

// CSRFMiddleware enforces anti-CSRF token verification on protected state-changing methods
func CSRFMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookieToken, err := c.Cookie(CSRFCookieName)

		// Generate token if not present or invalid
		if err != nil || cookieToken == "" || !ValidateCSRFToken(cookieToken, cfg.CSRFSecret) {
			cookieToken = GenerateCSRFToken(cfg.CSRFSecret)
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie(
				CSRFCookieName,
				cookieToken,
				86400, // 24 hours
				"/",
				cfg.CookieDomain,
				cfg.CookieSecure,
				false, // Accessible via JS so frontend can include it in X-CSRF-Token header
			)
		}

		// Always attach token to response header for convenience
		c.Header(CSRFHeaderName, cookieToken)

		path := c.Request.URL.Path
		// Exempt login, register, OTP sending & OTP verification, account
		// activation/resend, and MFA setup endpoints from CSRF header checks —
		// none of these have a session yet to carry a meaningful CSRF cookie.
		if strings.HasSuffix(path, "/register") ||
			strings.HasSuffix(path, "/login") ||
			strings.HasSuffix(path, "/verify-otp") ||
			strings.HasSuffix(path, "/send-otp") ||
			strings.Contains(path, "/otp") ||
			strings.Contains(path, "/activate") ||
			strings.Contains(path, "/resend-activation") ||
			strings.Contains(path, "/mfa/") ||
			path == "/api/auth/csrf" {
			c.Next()
			return
		}

		// State-altering HTTP methods on protected routes require token verification
		method := c.Request.Method
		if method == "POST" || method == "PUT" || method == "DELETE" || method == "PATCH" {
			headerToken := c.GetHeader(CSRFHeaderName)
			if headerToken == "" {
				headerToken = c.PostForm("csrf_token")
			}

			if headerToken == "" || headerToken != cookieToken || !ValidateCSRFToken(headerToken, cfg.CSRFSecret) {
				c.JSON(http.StatusForbidden, dto.APIError{
					Success: false,
					Error:   "CSRF token validation failed. Missing or invalid X-CSRF-Token header.",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
