package auth

import (
	"crypto/rsa"
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type UserRole string

const (
	AdminRole    UserRole = "admin"
	CustomerRole UserRole = "customer"
)

const (
	ScopeCarsReserve = "cars:reserve"
)

type Claims struct {
	UserID uint     `json:"user_id"`
	Role   UserRole `json:"role"`
	// Scope is a space-separated list of scopes (RFC 6749 section 3.3).
	Scope string `json:"scope"`
	// The standard "sub" claim is in RegisteredClaims.Subject: for a service
	// token it is the name of the calling service, e.g. "order-service".
	jwt.RegisteredClaims
}

type Identity struct {
	UserID  uint
	Role    UserRole
	Subject string
	Scope   string
}

// HasScope reports whether the space-separated scope list contains required.
// Scopes are compared as whole words: "cars:reserved" does not grant
// "cars:reserve".
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

func ParseJWTToken(publicKey *rsa.PublicKey, tokenString string) (Identity, error) {
	if publicKey == nil {
		return Identity{}, errors.New("public key not configured")
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
		return Identity{}, err
	}

	if !token.Valid {
		return Identity{}, errors.New("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return Identity{}, errors.New("unknown claims type, cannot parse the token")
	}

	return Identity{
		UserID:  claims.UserID,
		Role:    claims.Role,
		Subject: claims.Subject,
		Scope:   claims.Scope,
	}, nil
}
