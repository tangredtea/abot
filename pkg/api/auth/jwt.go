// Package auth provides JWT-based authentication for the web console.
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig holds JWT signing configuration.
type JWTConfig struct {
	Secret string
	Expiry time.Duration
}

// Claims contains the JWT payload for an authenticated user.
type Claims struct {
	AccountID string   `json:"account_id"`
	Role      string   `json:"role"`
	Tenants   []string `json:"tenants"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT from the given claims.
func GenerateToken(cfg JWTConfig, accountID, role string, tenants []string) (string, error) {
	now := time.Now()
	expiry := cfg.Expiry
	if expiry == 0 {
		expiry = 24 * time.Hour
	}
	claims := &Claims{
		AccountID: accountID,
		Role:      role,
		Tenants:   tenants,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "abot-console",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// ValidateToken parses and validates a JWT string, returning the embedded claims.
func ValidateToken(cfg JWTConfig, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
