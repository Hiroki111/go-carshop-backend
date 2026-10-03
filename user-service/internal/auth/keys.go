package auth

import (
	"crypto/rsa"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// LoadPrivateKey reads a PEM-encoded RSA private key from disk. Only
// user-service should ever load a private key: it's the one service
// allowed to mint tokens. That's the whole point of moving from a shared
// HMAC secret (where any service holding it could both sign and verify
// tokens, including forging an admin one) to RSA, where signing and
// verifying use two different keys.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to read private key file: %w", err)
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to parse private key: %w", err)
	}

	return key, nil
}

// LoadPublicKey reads a PEM-encoded RSA public key from disk. Clients(car-service,
// order-service, etc) each keep their own copy of a function like this (and
// their own copy of the public key file) so they can verify tokens without
// ever being able to mint one themselves.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to read public key file: %w", err)
	}

	key, err := jwt.ParseRSAPublicKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to parse public key: %w", err)
	}

	return key, nil
}
