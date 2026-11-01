package main

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	customerID  = uint(1)
	adminUserID = uint(2)
)

func lookupPath(userID uint) string {
	return fmt.Sprintf("/internal/users/%d", userID)
}

func seedLookupUsers(t *testing.T, db *gorm.DB) {
	t.Helper()

	users := []domain.User{
		{Model: gorm.Model{ID: customerID}, UserName: "alice", Email: "alice@example.com", Password: "not-a-real-hash", Role: domain.CustomerRole},
		{Model: gorm.Model{ID: adminUserID}, UserName: "admin", Email: "admin@example.com", Password: "not-a-real-hash", Role: domain.AdminRole},
	}
	require.NoError(t, db.Create(&users).Error)
}

func signServiceToken(t *testing.T, key *rsa.PrivateKey, scope string) string {
	t.Helper()

	token, err := auth.GenerateServiceToken(key, "message-service", strings.Fields(scope))
	require.NoError(t, err)
	return token
}

func expiredServiceToken(t *testing.T, key *rsa.PrivateKey, scope string) string {
	t.Helper()

	claims := auth.ServiceClaims{
		Scope: scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "message-service",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	require.NoError(t, err)
	return token
}

func userToken(t *testing.T, key *rsa.PrivateKey, role domain.UserRole) string {
	t.Helper()

	token, err := auth.GenerateJWTToken(key, 99, "someone", role)
	require.NoError(t, err)
	return token
}

func TestGetUserForService(t *testing.T) {
	otherKey := mustGenerateKey()
	aliceItem := map[string]any{"id": float64(customerID), "user_name": "alice", "email": "alice@example.com"}
	adminItem := map[string]any{"id": float64(adminUserID), "user_name": "admin", "email": "admin@example.com"}

	tests := []struct {
		testName     string
		token        string
		path         string
		expectedCode int
		expectedItem map[string]any
	}{
		{
			testName: "success - customer", token: signServiceToken(t, testKey, "users:read"),
			path: lookupPath(customerID), expectedCode: http.StatusOK, expectedItem: aliceItem,
		},
		{
			testName: "success - admin user", token: signServiceToken(t, testKey, "users:read"),
			path: lookupPath(adminUserID), expectedCode: http.StatusOK, expectedItem: adminItem,
		},
		{
			testName: "success - token with several scopes", token: signServiceToken(t, testKey, "cars:reserve users:read"),
			path: lookupPath(customerID), expectedCode: http.StatusOK, expectedItem: aliceItem,
		},

		{testName: "fail - no token", token: "", path: lookupPath(customerID), expectedCode: http.StatusUnauthorized},
		{testName: "fail - garbage token", token: "not-a-jwt", path: lookupPath(customerID), expectedCode: http.StatusUnauthorized},
		{
			testName: "fail - expired token", token: expiredServiceToken(t, testKey, "users:read"),
			path: lookupPath(customerID), expectedCode: http.StatusUnauthorized,
		},
		{
			testName: "fail - token signed by another key", token: signServiceToken(t, otherKey, "users:read"),
			path: lookupPath(customerID), expectedCode: http.StatusUnauthorized,
		},
		{
			testName: "fail - customer's user token", token: userToken(t, testKey, domain.CustomerRole),
			path: lookupPath(customerID), expectedCode: http.StatusForbidden,
		},
		{
			testName: "fail - admin's user token", token: userToken(t, testKey, domain.AdminRole),
			path: lookupPath(customerID), expectedCode: http.StatusForbidden,
		},
		{
			testName: "fail - service token without scopes", token: signServiceToken(t, testKey, ""),
			path: lookupPath(customerID), expectedCode: http.StatusForbidden,
		},
		{
			testName: "fail - service token with another scope", token: signServiceToken(t, testKey, "cars:reserve"),
			path: lookupPath(customerID), expectedCode: http.StatusForbidden,
		},
		{
			testName: "fail - service token with a look-alike scope", token: signServiceToken(t, testKey, "users:reader"),
			path: lookupPath(customerID), expectedCode: http.StatusForbidden,
		},

		{
			testName: "fail - unknown user", token: signServiceToken(t, testKey, "users:read"),
			path: lookupPath(999), expectedCode: http.StatusNotFound,
		},
		{
			testName: "fail - malformed ID", token: signServiceToken(t, testKey, "users:read"),
			path: "/internal/users/abc", expectedCode: http.StatusBadRequest,
		},
		{
			testName: "fail - ID is zero", token: signServiceToken(t, testKey, "users:read"),
			path: "/internal/users/0", expectedCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			app, db := setupTestApp(t)
			seedLookupUsers(t, db)

			rec := executeRequestWithToken(t, app, http.MethodGet, test.path, test.token, nil)

			require.Equal(t, test.expectedCode, rec.Code)
			if test.expectedCode == http.StatusOK {
				require.Equal(t, test.expectedItem, decodeBody[map[string]map[string]any](t, rec)["item"])
				return
			}
			require.NotContains(t, rec.Body.String(), "@example.com", "a rejected request must not reveal an email")
		})
	}
}
