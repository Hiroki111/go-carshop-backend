package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type UserRole string

const (
	AdminRole    UserRole = "admin"
	CustomerRole UserRole = "customer"
)

const tokenTTL = 24 * time.Hour

type Claims struct {
	UserID uint     `json:"user_id"`
	Role   UserRole `json:"role"`
	jwt.RegisteredClaims
}

func getSecretKey() ([]byte, error) {
	key := os.Getenv("SECRET_KEY")
	if key == "" {
		return nil, errors.New("SECRET_KEY not set")
	}
	return []byte(key), nil
}

func GenerateJWTToken(userID uint, role UserRole) (string, error) {
	secretKey, err := getSecretKey()
	if err != nil {
		return "", err
	}

	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secretKey)
}

func ParseJWTToken(tokenString string) (uint, UserRole, error) {
	secretKey, err := getSecretKey()
	if err != nil {
		return 0, "", err
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return secretKey, nil
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
