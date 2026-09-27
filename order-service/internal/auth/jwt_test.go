package auth

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func setSecretKey(t *testing.T) {
	t.Helper()
	err := os.Setenv("SECRET_KEY", "test-secret")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = os.Unsetenv("SECRET_KEY")
	})
}

func TestParseJWTToken_MalformedToken(t *testing.T) {
	setSecretKey(t)

	_, _, err := ParseJWTToken("this.is.not.a.jwt")
	require.Error(t, err)
}

func TestParseJWTToken_ExpiredToken(t *testing.T) {
	setSecretKey(t)

	secretKey := []byte("test-secret")

	claims := Claims{
		UserID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secretKey)
	require.NoError(t, err)

	_, _, err = ParseJWTToken(tokenString)
	require.Error(t, err)
}

func TestParseJWTToken_ValidToken(t *testing.T) {
	setSecretKey(t)

	userID := uint(1)
	role := AdminRole

	tokenString, err := GenerateJWTToken(userID, role)
	require.NoError(t, err)

	parsedUserID, parsedRole, err := ParseJWTToken(tokenString)
	require.NoError(t, err)
	require.Equal(t, userID, parsedUserID)
	require.Equal(t, role, parsedRole)
}
