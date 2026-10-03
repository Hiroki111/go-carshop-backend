package auth

import (
	"crypto/rsa"
	"errors"
	"time"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 24 * time.Hour

type Claims struct {
	UserID uint            `json:"user_id"`
	Role   domain.UserRole `json:"role"`
	jwt.RegisteredClaims
}

func GenerateJWTToken(privateKey *rsa.PrivateKey, userID uint, role domain.UserRole) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

// ParseJWTToken is here mainly as a template: Clients (order-service, car-service, etc)
// will each get their own near-identical copy of this (with their own
// locally-defined UserRole type, and only ever a public key) since they're
// the ones that actually need to verify incoming tokens. user-service has
// no token-protected routes yet, so nothing calls this one today.
func ParseJWTToken(publicKey *rsa.PublicKey, tokenString string) (uint, domain.UserRole, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return publicKey, nil
		})
	if err != nil {
		return 0, "", err
	}

	if !token.Valid {
		return 0, "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return 0, "", errors.New("unknown claims type, cannot parse the token")
	}

	return claims.UserID, claims.Role, nil
}
