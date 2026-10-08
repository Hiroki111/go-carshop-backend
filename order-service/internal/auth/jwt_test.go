package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func getKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey
	return privateKey, publicKey
}

func TestParseJWTToken_MalformedToken(t *testing.T) {
	_, publicKey := getKeys(t)

	_, err := ParseJWTToken(publicKey, "this.is.not.a.jwt")
	require.Error(t, err)
}

func TestParseJWTToken_ExpiredToken(t *testing.T) {
	privateKey, publicKey := getKeys(t)

	claims := Claims{
		UserID:   1,
		UserName: "alice",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)

	_, err = ParseJWTToken(publicKey, tokenString)
	require.ErrorIs(t, err, jwt.ErrTokenExpired)
}

func TestParseJWTToken_NilKey(t *testing.T) {
	privateKey, _ := getKeys(t)

	claims := Claims{
		UserID:   1,
		UserName: "alice",
		Role:     AdminRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Minute)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)

	_, err = ParseJWTToken(nil, tokenString)
	require.Error(t, err)
}

func TestParseJWTToken_ValidToken(t *testing.T) {
	privateKey, publicKey := getKeys(t)

	claims := Claims{
		UserID:   1,
		UserName: "alice",
		Role:     AdminRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Minute)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)

	identity, err := ParseJWTToken(publicKey, tokenString)
	require.NoError(t, err)
	require.Equal(t, Identity{UserID: 1, UserName: "alice", Role: AdminRole}, identity)
}

func TestParseJWTToken_RejectsOtherRSAAlgorithms(t *testing.T) {
	privateKey, publicKey := getKeys(t)
	claims := Claims{
		UserID: 1, UserName: "alice", Role: AdminRole,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))},
	}
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodRS512, claims).SignedString(privateKey)
	require.NoError(t, err)

	_, err = ParseJWTToken(publicKey, tokenString)
	require.Error(t, err)
}

func TestParseJWTToken_RejectsMissingExpiry(t *testing.T) {
	privateKey, publicKey := getKeys(t)
	claims := Claims{UserID: 1, UserName: "alice", Role: AdminRole}
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	require.NoError(t, err)

	_, err = ParseJWTToken(publicKey, tokenString)
	require.Error(t, err)
}
