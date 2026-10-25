package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/handler"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/testutil"
)

func releasePath(carID uint) string {
	return fmt.Sprintf("/internal/cars/%d/release", carID)
}

func TestReleaseCar(t *testing.T) {
	const orderID = 42

	tests := []struct {
		testName        string
		from            domain.CarStatus
		heldBy          *uint // the order that already holds the car, if any
		body            any
		carIDOffset     uint   // added to the seeded car's ID; non-zero asks for a car that does not exist
		pathOverride    string // used instead of the car's path when set
		expectedCode    int
		expectedStatus  domain.CarStatus // the seeded car's status afterwards
		expectedHeldBy  *uint            // the order holding the car afterwards
		expectedVersion uint
	}{
		{
			testName: "reserved car is released", from: domain.CarStatusReserved, heldBy: uintPtr(orderID),
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusOK,
			expectedStatus: domain.CarStatusAvailable, expectedHeldBy: nil, expectedVersion: 2,
		},
		{
			testName: "releasing the same car again is not an error", from: domain.CarStatusAvailable, heldBy: nil,
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusOK,
			expectedStatus: domain.CarStatusAvailable, expectedHeldBy: nil, expectedVersion: 1,
		},
		{
			testName: "fail - release request for another order", from: domain.CarStatusReserved, heldBy: uintPtr(7),
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusConflict,
			expectedStatus: domain.CarStatusReserved, expectedHeldBy: uintPtr(7), expectedVersion: 1,
		},
		{
			testName: "fail - sold by the same order", from: domain.CarStatusSold, heldBy: uintPtr(orderID),
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusConflict,
			expectedStatus: domain.CarStatusSold, expectedHeldBy: uintPtr(orderID), expectedVersion: 1,
		},
		{
			testName: "fail - sold by a different order", from: domain.CarStatusSold, heldBy: uintPtr(7),
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusConflict,
			expectedStatus: domain.CarStatusSold, expectedHeldBy: uintPtr(7), expectedVersion: 1,
		},
		{
			testName: "fail - withdrawn", from: domain.CarStatusWithdrawn,
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusConflict,
			expectedStatus: domain.CarStatusWithdrawn, expectedVersion: 1,
		},
		{
			testName: "fail - car not found", from: domain.CarStatusReserved, carIDOffset: 1000,
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusNotFound,
			expectedStatus: domain.CarStatusReserved, expectedVersion: 1,
		},
		{
			testName: "fail - invalid ID", from: domain.CarStatusReserved, pathOverride: "/internal/cars/abc/release",
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusReserved, expectedVersion: 1,
		},
		{
			testName: "fail - ID is zero", from: domain.CarStatusReserved, pathOverride: "/internal/cars/0/release",
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusReserved, expectedVersion: 1,
		},
		{
			testName: "fail - missing order_id", from: domain.CarStatusReserved,
			body: map[string]any{}, expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusReserved, expectedVersion: 1,
		},
		{
			testName: "fail - order_id is zero", from: domain.CarStatusReserved,
			body: map[string]any{"order_id": 0}, expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusReserved, expectedVersion: 1,
		},
		{
			testName: "fail - order_id is negative", from: domain.CarStatusReserved,
			body: map[string]any{"order_id": -1}, expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusReserved, expectedVersion: 1,
		},
		{
			testName: "fail - malformed body", from: domain.CarStatusReserved,
			body: "not an object", expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusReserved, expectedVersion: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			app, db := setupTestApp(t)

			car := domain.Car{Name: "test", PriceCents: 5, Status: test.from, ReservedByOrderID: test.heldBy}
			if err := db.Create(&car).Error; err != nil {
				t.Fatal(err)
			}

			path := releasePath(car.ID + test.carIDOffset)
			if test.pathOverride != "" {
				path = test.pathOverride
			}

			rec := executeRequest(t, app, http.MethodPost, path, serviceTokenFor(t, auth.ScopeCarsRelease), test.body)

			if rec.Code != test.expectedCode {
				t.Fatalf("expected code %d, got %d (body: %s)", test.expectedCode, rec.Code, rec.Body.String())
			}

			var stored domain.Car
			if err := db.First(&stored, car.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Status != test.expectedStatus {
				t.Fatalf("expected status %q in DB, got %q", test.expectedStatus, stored.Status)
			}
			if stored.Version != test.expectedVersion {
				t.Fatalf("expected version %d in DB, got %d", test.expectedVersion, stored.Version)
			}
			switch {
			case test.expectedHeldBy == nil && stored.ReservedByOrderID != nil:
				t.Fatalf("expected no order to hold the car, got order %d", *stored.ReservedByOrderID)
			case test.expectedHeldBy != nil && (stored.ReservedByOrderID == nil || *stored.ReservedByOrderID != *test.expectedHeldBy):
				t.Fatalf("expected order %d to hold the car, got %s", *test.expectedHeldBy, testutil.DescribeUint(stored.ReservedByOrderID))
			}

			if test.expectedCode == http.StatusOK {
				var resp handler.ReleaseCarResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Item.ID != car.ID || resp.Item.Status != string(domain.CarStatusAvailable) {
					t.Fatalf("unexpected response item: %+v", resp.Item)
				}
			}
		})
	}
}

func TestReleaseCar_Authorization(t *testing.T) {
	otherPrivateKey := newTestKey(t)

	tests := []struct {
		testName     string
		token        string
		expectedCode int
	}{
		{testName: "no token", token: "", expectedCode: http.StatusUnauthorized},
		{testName: "garbage token", token: "not-a-jwt", expectedCode: http.StatusUnauthorized},
		{
			testName:     "service token signed by an untrusted key",
			token:        signServiceToken(t, otherPrivateKey, "order-service", auth.ScopeCarsRelease, time.Now().Add(time.Hour)),
			expectedCode: http.StatusUnauthorized,
		},
		{
			testName:     "service token expired",
			token:        signServiceToken(t, trustedKey, "order-service", auth.ScopeCarsRelease, time.Now().Add(-time.Hour)),
			expectedCode: http.StatusUnauthorized,
		},
		{testName: "customer token", token: tokenFor(t, auth.CustomerRole), expectedCode: http.StatusForbidden},
		{testName: "admin token", token: tokenFor(t, auth.AdminRole), expectedCode: http.StatusForbidden},
		{testName: "service token without scopes", token: serviceTokenFor(t, ""), expectedCode: http.StatusForbidden},
		{testName: "service token with another scope", token: serviceTokenFor(t, "cars:reserve"), expectedCode: http.StatusForbidden},
		{testName: "service token with a look-alike scope", token: serviceTokenFor(t, "cars:reserved"), expectedCode: http.StatusForbidden},
		{testName: "service token with the scope", token: serviceTokenFor(t, auth.ScopeCarsRelease), expectedCode: http.StatusOK},
		{testName: "service token with several scopes", token: serviceTokenFor(t, "cars:release cars:reserve"), expectedCode: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			app, db := setupTestApp(t)

			car := domain.Car{Name: "test", PriceCents: 5, Status: domain.CarStatusReserved, ReservedByOrderID: uintPtr(1)}
			if err := db.Create(&car).Error; err != nil {
				t.Fatal(err)
			}

			rec := executeRequest(t, app, http.MethodPost, releasePath(car.ID), test.token, map[string]any{"order_id": 1})

			if rec.Code != test.expectedCode {
				t.Fatalf("expected code %d, got %d (body: %s)", test.expectedCode, rec.Code, rec.Body.String())
			}

			// A rejected caller must not change anything.
			var stored domain.Car
			if err := db.First(&stored, car.ID).Error; err != nil {
				t.Fatal(err)
			}
			wantStatus := domain.CarStatusReserved
			if test.expectedCode == http.StatusOK {
				wantStatus = domain.CarStatusAvailable
			}
			if stored.Status != wantStatus {
				t.Fatalf("expected status %q in DB, got %q", wantStatus, stored.Status)
			}
		})
	}
}
