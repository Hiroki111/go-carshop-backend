package auth

import (
	"crypto/rsa"
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

type UserRole string

const (
	AdminRole    UserRole = "admin"
	CustomerRole UserRole = "customer"
)

type Claims struct {
	UserID uint     `json:"user_id"`
	Role   UserRole `json:"role"`
	jwt.RegisteredClaims
}

func ParseJWTToken(publicKey *rsa.PublicKey, tokenString string) (uint, UserRole, error) {
	if publicKey == nil {
		return 0, "", errors.New("public key not configured")
	}

	token, err := jwt.ParseWithClaims(
		tokenString, &Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return publicKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
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
