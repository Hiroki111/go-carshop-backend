package auth

import (
	"crypto/rsa"
	"time"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 24 * time.Hour

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
