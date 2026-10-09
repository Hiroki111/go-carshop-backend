package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/auth"
	"github.com/golang-jwt/jwt/v5"
)

func newTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return key
}

func signRS256(t *testing.T, key *rsa.PrivateKey, userID uint, role auth.UserRole, expiresAt time.Time) string {
	t.Helper()

	claims := auth.Claims{
		UserID:           userID,
		Role:             role,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiresAt)},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func signHS256(t *testing.T) string {
	t.Helper()

	claims := auth.Claims{
		UserID:           1,
		Role:             auth.AdminRole,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func TestRequireToken(t *testing.T) {
	trustedKey := newTestKey(t)
	attackerKey := newTestKey(t)
	userID := uint(123)

	tests := []struct {
		name           string
		authHeader     string
		expectStatus   int
		expectNextCall bool
	}{
		{name: "missing Authorization header", expectStatus: http.StatusUnauthorized},
		{name: "malformed Authorization header", authHeader: "invalid", expectStatus: http.StatusUnauthorized},
		{name: "wrong scheme", authHeader: "Basic abc.def.ghi", expectStatus: http.StatusUnauthorized},
		{name: "invalid token", authHeader: "Bearer not-a-jwt", expectStatus: http.StatusUnauthorized},
		{
			name:         "expired token",
			authHeader:   "Bearer " + signRS256(t, trustedKey, userID, auth.AdminRole, time.Now().Add(-time.Hour)),
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "signed by a different key",
			authHeader:   "Bearer " + signRS256(t, attackerKey, userID, auth.AdminRole, time.Now().Add(time.Hour)),
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:         "HS256 token",
			authHeader:   "Bearer " + signHS256(t),
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:           "valid token",
			authHeader:     "Bearer " + signRS256(t, trustedKey, userID, auth.AdminRole, time.Now().Add(time.Hour)),
			expectStatus:   http.StatusOK,
			expectNextCall: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nextCalled := false
			next := func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)

				if got := r.Context().Value(UserIDKey); got != userID {
					t.Errorf("expected user ID %v in context, got %v", userID, got)
				}
				if got := r.Context().Value(RoleKey); got != auth.AdminRole {
					t.Errorf("expected role %q in context, got %v", auth.AdminRole, got)
				}
			}
			h := &Handler{publicKey: &trustedKey.PublicKey}

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if test.authHeader != "" {
				req.Header.Set("Authorization", test.authHeader)
			}
			rec := httptest.NewRecorder()

			h.RequireToken(next).ServeHTTP(rec, req)

			if rec.Code != test.expectStatus {
				t.Fatalf("expected status %d, got %d", test.expectStatus, rec.Code)
			}
			if nextCalled != test.expectNextCall {
				t.Fatalf("expected next handler called = %v, got %v", test.expectNextCall, nextCalled)
			}
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
			authHeader:     "Bearer " + signRS256(t, trustedKey, 1, auth.AdminRole, exp),
			expectStatus:   http.StatusOK,
			expectNextCall: true,
		},
		{
			name:         "customer",
			authHeader:   "Bearer " + signRS256(t, trustedKey, 1, auth.CustomerRole, exp),
			expectStatus: http.StatusForbidden,
		},
		{
			name:         "token with empty role",
			authHeader:   "Bearer " + signRS256(t, trustedKey, 1, "", exp),
			expectStatus: http.StatusForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nextCalled := false
			next := func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			}

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if test.authHeader != "" {
				req.Header.Set("Authorization", test.authHeader)
			}
			rec := httptest.NewRecorder()

			h.RequireRole(auth.AdminRole, next).ServeHTTP(rec, req)

			if rec.Code != test.expectStatus {
				t.Fatalf("expected status %d, got %d", test.expectStatus, rec.Code)
			}
			if nextCalled != test.expectNextCall {
				t.Fatalf("expected next handler called = %v, got %v", test.expectNextCall, nextCalled)
			}
		})
	}
}
