package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func newTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func signRS256(t *testing.T, key *rsa.PrivateKey, userID uint, userName string, role auth.UserRole, expiresAt time.Time) string {
	t.Helper()
	claims := auth.Claims{
		UserID:           userID,
		UserName:         userName,
		Role:             role,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiresAt)},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	require.NoError(t, err)
	return s
}

func signHS256(t *testing.T) string {
	t.Helper()
	claims := auth.Claims{
		UserID:           1,
		UserName:         "alice",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return s
}

func TestRequireToken(t *testing.T) {
	trustedKey := newTestKey(t)
	attackerKey := newTestKey(t)

	tests := []struct {
		name           string
		authHeader     string
		expectStatus   int
		expectNextCall bool
	}{
		{
			name:         "missing Authorization header",
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "malformed Authorization header",
			authHeader:   "invalid",
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "wrong scheme",
			authHeader:   "Basic abc.def.ghi",
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "invalid token",
			authHeader:   "Bearer not-a-jwt",
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "expired token",
			authHeader:   "Bearer " + signRS256(t, trustedKey, 1, "alice", auth.AdminRole, time.Now().Add(-time.Hour)),
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "signed by a different key",
			authHeader:   "Bearer " + signRS256(t, attackerKey, 1, "alice", auth.AdminRole, time.Now().Add(time.Hour)),
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "HS256 token",
			authHeader:   "Bearer " + signHS256(t),
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:           "valid token",
			authHeader:     "Bearer " + signRS256(t, trustedKey, 123, "alice", auth.AdminRole, time.Now().Add(time.Hour)),
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

				require.Equal(t, uint(123), r.Context().Value(UserIDKey))
				require.Equal(t, "alice", r.Context().Value(UserNameKey))
				require.Equal(t, auth.AdminRole, r.Context().Value(RoleKey))
			})
			h := &Handler{publicKey: &trustedKey.PublicKey}
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

func TestRequireRole_AdminOnlyRoute(t *testing.T) {
	trustedKey := newTestKey(t)
	h := &Handler{publicKey: &trustedKey.PublicKey}
	exp := time.Now().Add(time.Hour)

	tests := []struct {
		name           string
		authHeader     string
		expectStatus   int
		expectNextCall bool
	}{
		{name: "no token", expectStatus: http.StatusUnauthorized},
		{
			name:           "admin",
			authHeader:     "Bearer " + signRS256(t, trustedKey, 1, "alice", auth.AdminRole, exp),
			expectStatus:   http.StatusOK,
			expectNextCall: true,
		},
		{
			name:         "customer",
			authHeader:   "Bearer " + signRS256(t, trustedKey, 1, "alice", auth.CustomerRole, exp),
			expectStatus: http.StatusForbidden,
		},
		{
			name:         "token with empty role",
			authHeader:   "Bearer " + signRS256(t, trustedKey, 1, "alice", "", exp),
			expectStatus: http.StatusForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if test.authHeader != "" {
				req.Header.Set("Authorization", test.authHeader)
			}
			rec := httptest.NewRecorder()

			h.RequireRole(auth.AdminRole, next).ServeHTTP(rec, req)

			require.Equal(t, test.expectStatus, rec.Code)
			require.Equal(t, test.expectNextCall, nextCalled)
		})
	}
}
