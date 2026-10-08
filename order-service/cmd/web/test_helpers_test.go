package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient/carclienttest"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/handler"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

const defaultTestUserName = "test user"

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

func tokenFor(t *testing.T, key *rsa.PrivateKey, userID uint, role auth.UserRole) string {
	t.Helper()
	return signRS256(t, key, userID, defaultTestUserName, role, time.Now().Add(time.Hour))
}

// setupTestApp wires up the app for tests and returns the app, its database,
// and the private key whose public half the app trusts. Tokens must be signed
// with that key to be accepted.
//
// Pass nil for carClient to get the default carclienttest.FakeCarClient
// (every car exists and is available); pass a *carclienttest.FakeCarClient
// with GetCarByIDFunc set to exercise a specific scenario.
func setupTestApp(t *testing.T, carClient carclient.CarClient) (http.Handler, *gorm.DB, *rsa.PrivateKey) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := repository.NewRepository(db)
	require.NoError(t, repo.Migrate())

	if carClient == nil {
		carClient = &carclienttest.FakeCarClient{}
	}

	svc := service.NewService(repo, carClient)
	trustedKey := newTestKey(t)
	h := handler.NewHandler(svc, &trustedKey.PublicKey)

	return routes(h), db, trustedKey
}

func executeRequest(
	t *testing.T,
	app http.Handler,
	method, path, token string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var v T
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&v))
	return v
}

func seedOrders(t *testing.T, db *gorm.DB, orders []domain.Order) []domain.Order {
	t.Helper()

	base := time.Now()
	seededOrders := make([]domain.Order, len(orders))
	for i, order := range orders {
		order.CreatedAt = base.Add(time.Duration(i) * time.Second)
		require.NoError(t, db.Create(&order).Error)
		seededOrders[i] = order
	}

	return seededOrders
}
