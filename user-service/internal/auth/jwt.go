package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"errors"
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

type ServiceIdentity struct {
	// Subject is the client ID, e.g. "message-service".
	Subject string
	Scope   string
}

func ParseServiceToken(publicKey *rsa.PublicKey, tokenString string) (ServiceIdentity, error) {
	if publicKey == nil {
		return ServiceIdentity{}, errors.New("public key not configured")
	}

	token, err := jwt.ParseWithClaims(
		tokenString, &ServiceClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return publicKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return ServiceIdentity{}, err
	}

	claims, ok := token.Claims.(*ServiceClaims)
	if !ok || !token.Valid {
		return ServiceIdentity{}, errors.New("invalid token")
	}

	return ServiceIdentity{Subject: claims.Subject, Scope: claims.Scope}, nil
}

func HasScope(scope, required string) bool {
	if required == "" {
		return false
	}
	for _, s := range strings.Fields(scope) {
		if s == required {
			return true
		}
	}
	return false
}
