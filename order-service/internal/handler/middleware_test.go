package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/auth"
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

func validToken(t *testing.T, userID uint, role auth.UserRole) string {
	t.Helper()

	token, err := auth.GenerateJWTToken(userID, role)
	require.NoError(t, err)

	return token
}

func expiredToken(t *testing.T) string {
	t.Helper()

	claims := auth.Claims{
		UserID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	return tokenString
}

func TestRequireToken(t *testing.T) {
	setSecretKey(t)

	tests := []struct {
		name           string
		authHeader     string
		expectStatus   int
		expectNextCall bool
	}{
		{
			name:           "missing Authorization header",
			expectStatus:   http.StatusUnauthorized,
			expectNextCall: false,
		},
		{
			name:           "malformed Authorization header",
			authHeader:     "invalid",
			expectStatus:   http.StatusUnauthorized,
			expectNextCall: false,
		},
		{
			name:           "wrong scheme",
			authHeader:     "Basic abc.def.ghi",
			expectStatus:   http.StatusUnauthorized,
			expectNextCall: false,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer not-a-jwt",
			expectStatus:   http.StatusUnauthorized,
			expectNextCall: false,
		},
		{
			name:           "expired token",
			authHeader:     "Bearer " + expiredToken(t),
			expectStatus:   http.StatusUnauthorized,
			expectNextCall: false,
		},
		{
			name:           "valid token",
			authHeader:     "Bearer " + validToken(t, 123, auth.AdminRole),
			expectStatus:   http.StatusOK,
			expectNextCall: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nextCalled := false

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})
			h := &Handler{}
			handler := h.RequireToken(next)

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if test.authHeader != "" {
				req.Header.Set("Authorization", test.authHeader)
			}

			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			require.Equal(t, test.expectStatus, rec.Code)
			require.Equal(t, test.expectNextCall, nextCalled)
		})
	}
}
