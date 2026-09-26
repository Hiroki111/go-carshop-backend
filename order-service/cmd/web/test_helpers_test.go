package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/handler"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestApp(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	repo := repository.NewRepository(db)

	if err := repo.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	service := service.NewService(repo)

	handler := handler.NewHandler(service)
	return routes(handler), db
}

func executeRequest(
	t *testing.T,
	app http.Handler,
	method, path string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("failed to encode body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	return rec
}

func strPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

func seedOrders(t *testing.T, db *gorm.DB, orders []domain.Order) []domain.Order {
	t.Helper()

	seededOrders := make([]domain.Order, len(orders))
	for i, order := range orders {
		order.CreatedAt = time.Now().Add(time.Duration(i) * time.Second)
		if result := db.Create(&order); result.Error != nil {
			t.Fatal(result.Error)
		}
		seededOrders[i] = order
	}

	return seededOrders
}
