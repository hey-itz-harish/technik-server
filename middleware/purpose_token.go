package middleware

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// PurposeClaims backs short-lived, single-purpose signed tokens that are
// distinct from full session JWTClaims (see auth.go). They are used to
// authorize a narrow follow-up action (resending an activation email,
// completing MFA setup) without requiring a logged-in session.
type PurposeClaims struct {
	Email   string `json:"email"`
	Type    string `json:"type"`    // "school" or "student"
	Purpose string `json:"purpose"` // e.g. "resend-activation", "mfa-setup"
	jwt.RegisteredClaims
}

// GeneratePurposeToken creates a signed, single-purpose token valid for ttl.
func GeneratePurposeToken(email, entityType, purpose, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := PurposeClaims{
		Email:   email,
		Type:    entityType,
		Purpose: purpose,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "technik-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidatePurposeToken verifies the token's signature, expiry, and that its
// Purpose claim matches expectedPurpose.
func ValidatePurposeToken(tokenStr, expectedPurpose, secret string) (*PurposeClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &PurposeClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*PurposeClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}

	if claims.Purpose != expectedPurpose {
		return nil, errors.New("token purpose mismatch")
	}

	return claims, nil
}
