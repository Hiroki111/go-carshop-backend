package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/handler"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/testutil"
)

func reservationPath(carID uint) string {
	return fmt.Sprintf("/internal/cars/%d/reservations", carID)
}

func uintPtr(u uint) *uint {
	return &u
}

func TestReserveCar(t *testing.T) {
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
			testName: "available car is reserved", from: domain.CarStatusAvailable,
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusOK,
			expectedStatus: domain.CarStatusReserved, expectedHeldBy: uintPtr(orderID), expectedVersion: 2,
		},
		{
			testName: "same order asking again is not an error", from: domain.CarStatusReserved, heldBy: uintPtr(orderID),
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusOK,
			expectedStatus: domain.CarStatusReserved, expectedHeldBy: uintPtr(orderID), expectedVersion: 1,
		},
		{
			testName: "fail - reserved for another order", from: domain.CarStatusReserved, heldBy: uintPtr(7),
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusConflict,
			expectedStatus: domain.CarStatusReserved, expectedHeldBy: uintPtr(7), expectedVersion: 1,
		},
		{
			testName: "fail - sold", from: domain.CarStatusSold, heldBy: uintPtr(7),
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusConflict,
			expectedStatus: domain.CarStatusSold, expectedHeldBy: uintPtr(7), expectedVersion: 1,
		},
		{
			// Even the order that bought it can't reserve a sold car again.
			testName: "fail - sold to the same order", from: domain.CarStatusSold, heldBy: uintPtr(orderID),
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusConflict,
			expectedStatus: domain.CarStatusSold, expectedHeldBy: uintPtr(orderID), expectedVersion: 1,
		},
		{
			testName: "fail - withdrawn", from: domain.CarStatusWithdrawn,
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusConflict,
			expectedStatus: domain.CarStatusWithdrawn, expectedVersion: 1,
		},

		{
			testName: "fail - car not found", from: domain.CarStatusAvailable, carIDOffset: 1000,
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusNotFound,
			expectedStatus: domain.CarStatusAvailable, expectedVersion: 1,
		},
		{
			testName: "fail - invalid ID", from: domain.CarStatusAvailable, pathOverride: "/internal/cars/abc/reservations",
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusAvailable, expectedVersion: 1,
		},
		{
			testName: "fail - ID is zero", from: domain.CarStatusAvailable, pathOverride: "/internal/cars/0/reservations",
			body: map[string]any{"order_id": orderID}, expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusAvailable, expectedVersion: 1,
		},
		{
			testName: "fail - missing order_id", from: domain.CarStatusAvailable,
			body: map[string]any{}, expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusAvailable, expectedVersion: 1,
		},
		{
			testName: "fail - order_id is zero", from: domain.CarStatusAvailable,
			body: map[string]any{"order_id": 0}, expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusAvailable, expectedVersion: 1,
		},
		{
			testName: "fail - order_id is negative", from: domain.CarStatusAvailable,
			body: map[string]any{"order_id": -1}, expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusAvailable, expectedVersion: 1,
		},
		{
			testName: "fail - malformed body", from: domain.CarStatusAvailable,
			body: "not an object", expectedCode: http.StatusBadRequest,
			expectedStatus: domain.CarStatusAvailable, expectedVersion: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			app, db := setupTestApp(t)

			car := domain.Car{Name: "test", PriceCents: 5, Status: test.from, ReservedByOrderID: test.heldBy}
			if err := db.Create(&car).Error; err != nil {
				t.Fatal(err)
			}

			path := reservationPath(car.ID + test.carIDOffset)
			if test.pathOverride != "" {
				path = test.pathOverride
			}

			rec := executeRequest(t, app, http.MethodPost, path, serviceTokenFor(t, auth.ScopeCarsReserve), test.body)

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
				var resp handler.ReserveCarResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Item.ID != car.ID || resp.Item.Status != string(domain.CarStatusReserved) {
					t.Fatalf("unexpected response item: %+v", resp.Item)
				}
			}
		})
	}
}

func TestReserveCar_Authorization(t *testing.T) {
	otherKey := newTestKey(t)

	tests := []struct {
		testName     string
		token        string
		expectedCode int
	}{
		{testName: "no token", token: "", expectedCode: http.StatusUnauthorized},
		{testName: "garbage token", token: "not-a-jwt", expectedCode: http.StatusUnauthorized},
		{
			testName:     "service token signed by an untrusted key",
			token:        signServiceToken(t, otherKey, "order-service", auth.ScopeCarsReserve, time.Now().Add(time.Hour)),
			expectedCode: http.StatusUnauthorized,
		},
		{
			testName:     "service token expired",
			token:        signServiceToken(t, trustedKey, "order-service", auth.ScopeCarsReserve, time.Now().Add(-time.Hour)),
			expectedCode: http.StatusUnauthorized,
		},
		// Whoever the caller is, a user can't reserve a car through the internal
		// endpoint: a customer's order has to go through order-service.
		{testName: "customer token", token: tokenFor(t, auth.CustomerRole), expectedCode: http.StatusForbidden},
		{testName: "admin token", token: tokenFor(t, auth.AdminRole), expectedCode: http.StatusForbidden},
		{testName: "service token without scopes", token: serviceTokenFor(t, ""), expectedCode: http.StatusForbidden},
		{testName: "service token with another scope", token: serviceTokenFor(t, "cars:release"), expectedCode: http.StatusForbidden},
		{testName: "service token with a look-alike scope", token: serviceTokenFor(t, "cars:reserved"), expectedCode: http.StatusForbidden},
		{testName: "service token with the scope", token: serviceTokenFor(t, auth.ScopeCarsReserve), expectedCode: http.StatusOK},
		{testName: "service token with several scopes", token: serviceTokenFor(t, "cars:release cars:reserve"), expectedCode: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			app, db := setupTestApp(t)

			car := domain.Car{Name: "test", PriceCents: 5}
			if err := db.Create(&car).Error; err != nil {
				t.Fatal(err)
			}

			rec := executeRequest(t, app, http.MethodPost, reservationPath(car.ID), test.token, map[string]any{"order_id": 1})

			if rec.Code != test.expectedCode {
				t.Fatalf("expected code %d, got %d (body: %s)", test.expectedCode, rec.Code, rec.Body.String())
			}

			// A rejected caller must not change anything.
			var stored domain.Car
			if err := db.First(&stored, car.ID).Error; err != nil {
				t.Fatal(err)
			}
			wantStatus := domain.CarStatusAvailable
			if test.expectedCode == http.StatusOK {
				wantStatus = domain.CarStatusReserved
			}
			if stored.Status != wantStatus {
				t.Fatalf("expected status %q in DB, got %q", wantStatus, stored.Status)
			}
		})
	}
}

// A service token must not open the admin routes, and the other way round.
func TestServiceToken_CannotUseAdminRoutes(t *testing.T) {
	app, db := setupTestApp(t)

	car := domain.Car{Name: "test", PriceCents: 5}
	if err := db.Create(&car).Error; err != nil {
		t.Fatal(err)
	}

	token := serviceTokenFor(t, auth.ScopeCarsReserve)

	rec := executeRequest(t, app, http.MethodPatch, fmt.Sprintf("/cars/%d/status", car.ID), token, map[string]any{"status": "withdrawn"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected code %d, got %d (body: %s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}

	rec = executeRequest(t, app, http.MethodDelete, fmt.Sprintf("/cars/%d", car.ID), token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected code %d, got %d (body: %s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestReserveCar_UpdatesWhatCustomersSee(t *testing.T) {
	app, db := setupTestApp(t)

	seeded := seedCars(t, db, []domain.Car{
		{Name: "first", PriceCents: 100},
		{Name: "second", PriceCents: 200},
	})

	rec := executeRequest(t, app, http.MethodPost, reservationPath(seeded[0].ID), serviceTokenFor(t, auth.ScopeCarsReserve), map[string]any{"order_id": 1})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected code %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	rec = executeRequest(t, app, http.MethodGet, "/cars", "", nil)
	var list handler.GetCarsResponse
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != seeded[1].ID {
		t.Fatalf("expected only the second car to be listed, got %+v", list.Items)
	}

	rec = executeRequest(t, app, http.MethodGet, fmt.Sprintf("/cars/%d", seeded[0].ID), "", nil)
	var one handler.GetCarResponse
	if err := json.NewDecoder(rec.Body).Decode(&one); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if one.Item.Status != string(domain.CarStatusReserved) {
		t.Fatalf("expected status %q, got %q", domain.CarStatusReserved, one.Item.Status)
	}
}

func postReservation(app http.Handler, token string, carID, orderID uint) int {
	body, _ := json.Marshal(map[string]any{"order_id": orderID})

	req := httptest.NewRequest(http.MethodPost, reservationPath(carID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	return rec.Code
}

// The scenario this whole feature exists for: many customers press "buy" on
// the same car at the same moment. Needs a real Postgres (TEST_POSTGRES_DSN).
func TestReserveCar_ConcurrentRequestsOnlyOneWins(t *testing.T) {
	const attempts = 20

	db := testutil.NewPostgresDB(t)

	// One connection per in-flight request, so the requests really overlap.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(attempts + 5)

	app := newTestApp(t, db)

	car := domain.Car{Name: "the last one", PriceCents: 100}
	if err := db.Create(&car).Error; err != nil {
		t.Fatal(err)
	}

	token := serviceTokenFor(t, auth.ScopeCarsReserve)

	codes := make([]int, attempts)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = postReservation(app, token, car.ID, uint(i+1))
		}(i)
	}
	close(start)
	wg.Wait()

	winner := -1
	conflicts := 0
	for i, code := range codes {
		switch code {
		case http.StatusOK:
			if winner != -1 {
				t.Errorf("more than one request got the car: attempts %d and %d", winner, i)
			}
			winner = i
		case http.StatusConflict:
			conflicts++
		default:
			t.Errorf("attempt %d: unexpected status code %d", i, code)
		}
	}
	if winner == -1 {
		t.Fatal("expected exactly one request to get the car, none did")
	}
	if conflicts != attempts-1 {
		t.Errorf("expected %d conflicts, got %d", attempts-1, conflicts)
	}

	var stored domain.Car
	if err := db.First(&stored, car.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.CarStatusReserved {
		t.Errorf("expected status %q, got %q", domain.CarStatusReserved, stored.Status)
	}
	if stored.ReservedByOrderID == nil || *stored.ReservedByOrderID != uint(winner+1) {
		t.Errorf("expected the winning order %d to hold the car, got %s", winner+1, testutil.DescribeUint(stored.ReservedByOrderID))
	}
	if stored.Version != 2 {
		t.Errorf("expected version 2 (bumped once), got %d", stored.Version)
	}
}
