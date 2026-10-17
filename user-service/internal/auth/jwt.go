package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"strings"
	"time"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 24 * time.Hour

const ServiceTokenTTL = 5 * time.Minute

const serviceTokenIssuer = "user-service"

type Claims struct {
	UserID   uint            `json:"user_id"`
	UserName string          `json:"user_name"`
	Role     domain.UserRole `json:"role"`
	jwt.RegisteredClaims
}

func GenerateJWTToken(privateKey *rsa.PrivateKey, userID uint, userName string, role domain.UserRole) (string, error) {
	claims := Claims{
		UserID:   userID,
		UserName: userName,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

type ServiceClaims struct {
	// Scope is the space-separated list of granted scopes (RFC 6749 section 3.3).
	Scope string `json:"scope"`
	jwt.RegisteredClaims
}

func GenerateServiceToken(privateKey *rsa.PrivateKey, clientID string, scopes []string) (string, error) {
	tokenID := make([]byte, 16)
	if _, err := rand.Read(tokenID); err != nil {
		return "", err
	}

	now := time.Now()
	claims := ServiceClaims{
		Scope: strings.Join(scopes, " "),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    serviceTokenIssuer,
			Subject:   clientID,
			ID:        hex.EncodeToString(tokenID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ServiceTokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}
