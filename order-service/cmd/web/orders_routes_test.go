package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient/carclienttest"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/config"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/handler"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCreateOrder(t *testing.T) {
	const (
		userID   = uint(7)
		userName = "customer name"
		carID    = uint(123)
	)
	car := carclient.Car{ID: carID, Name: "Test Car", PriceCents: 100}

	tests := []struct {
		testName       string
		noToken        bool
		role           auth.UserRole
		body           any // defaults to a valid request for carID
		carErr         error
		isCarAvailable bool
		expectedCode   int
	}{
		{
			testName:       "success - customer token",
			role:           auth.CustomerRole,
			isCarAvailable: true,
			expectedCode:   http.StatusCreated,
		},
		{
			testName:       "success - admin token",
			role:           auth.AdminRole,
			isCarAvailable: true,
			expectedCode:   http.StatusCreated,
		},
		{
			testName:       "fail - no token",
			noToken:        true,
			isCarAvailable: true,
			expectedCode:   http.StatusUnauthorized,
		},
		{
			testName:     "fail - car not found",
			role:         auth.CustomerRole,
			carErr:       carclient.ErrCarNotFound,
			expectedCode: http.StatusNotFound,
		},
		{
			testName:       "fail - car not available",
			role:           auth.CustomerRole,
			isCarAvailable: false,
			expectedCode:   http.StatusConflict,
		},
		{
			testName:     "fail - car-service is failing",
			role:         auth.CustomerRole,
			carErr:       errors.New("car-service is down"),
			expectedCode: http.StatusInternalServerError,
		},
		{
			testName:       "fail - malformed body",
			role:           auth.CustomerRole,
			body:           "not an object",
			isCarAvailable: true,
			expectedCode:   http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			var requestedCarIDs []uint
			carClient := &carclienttest.FakeCarClient{
				GetCarByIDFunc: func(ctx context.Context, id uint) (carclient.Car, error) {
					requestedCarIDs = append(requestedCarIDs, id)
					if test.carErr != nil {
						return carclient.Car{}, test.carErr
					}
					c := car
					c.IsAvailable = test.isCarAvailable
					return c, nil
				},
			}
			app, db, trustedKey := setupTestApp(t, carClient)

			var token string
			if !test.noToken {
				token = signRS256(t, trustedKey, userID, userName, test.role, time.Now().Add(time.Hour))
			}

			body := test.body
			if body == nil {
				body = handler.CreateOrderRequest{CarID: carID}
			}

			rec := executeRequest(t, app, http.MethodPost, "/orders", token, body)
			require.Equal(t, test.expectedCode, rec.Code)

			var orders []domain.Order
			require.NoError(t, db.Find(&orders).Error)

			if test.expectedCode != http.StatusCreated {
				require.Empty(t, orders, "a rejected request must not create an order")
				return
			}

			require.Equal(t, "success", decodeBody[handler.CreateOrderResponse](t, rec).Message)
			require.Equal(t, []uint{carID}, requestedCarIDs)

			require.Len(t, orders, 1)
			require.Equal(t, userID, orders[0].UserID, "the order belongs to the token's user")
			require.Equal(t, userName, orders[0].UserName, "the order snapshots the token's user name")
			require.Equal(t, carID, orders[0].CarID)
			require.Equal(t, car.Name, orders[0].CarName)
			require.Equal(t, car.PriceCents, orders[0].PriceCents)
		})
	}
}

func TestGetOrders_WithSorting(t *testing.T) {
	app, db, trustedKey := setupTestApp(t, nil)
	token := tokenFor(t, trustedKey, 1, auth.AdminRole)

	// created_at ascending is the insertion order: cherry, apple, banana.
	// The car IDs and user IDs deliberately sort differently from created_at,
	// so that each orderBy value produces a distinguishable result (and the
	// default/fallback ordering can't be mistaken for a car_id or user_id sort).
	//
	// Ties (cherry and banana share user_id 1) are resolved by ID ascending,
	// i.e. insertion order.
	seedOrders(t, db, []domain.Order{
		{CarID: 3, CarName: "cherry", UserID: 1, UserName: "alice", PriceCents: 200},
		{CarID: 1, CarName: "apple", UserID: 2, UserName: "bob", PriceCents: 100},
		{CarID: 2, CarName: "banana", UserID: 1, UserName: "alice", PriceCents: 300},
	})
	userNameByCarName := map[string]string{"cherry": "alice", "apple": "bob", "banana": "alice"}

	tests := []struct {
		testName         string
		orderBy, sortIn  string
		expectedCarNames []string
	}{
		{testName: "car_id asc", orderBy: "car_id", sortIn: "asc", expectedCarNames: []string{"apple", "banana", "cherry"}},
		{testName: "car_id desc", orderBy: "car_id", sortIn: "desc", expectedCarNames: []string{"cherry", "banana", "apple"}},
		{testName: "car_id, default direction", orderBy: "car_id", expectedCarNames: []string{"apple", "banana", "cherry"}},
		{testName: "user_id asc", orderBy: "user_id", sortIn: "asc", expectedCarNames: []string{"cherry", "banana", "apple"}},
		{testName: "user_id desc", orderBy: "user_id", sortIn: "desc", expectedCarNames: []string{"apple", "cherry", "banana"}},
		{testName: "user_id, default direction", orderBy: "user_id", expectedCarNames: []string{"cherry", "banana", "apple"}},
		{testName: "created_at asc", orderBy: "created_at", sortIn: "asc", expectedCarNames: []string{"cherry", "apple", "banana"}},
		{testName: "created_at desc", orderBy: "created_at", sortIn: "desc", expectedCarNames: []string{"banana", "apple", "cherry"}},
		{testName: "created_at, default direction", orderBy: "created_at", expectedCarNames: []string{"cherry", "apple", "banana"}},
		{testName: "no parameters", expectedCarNames: []string{"cherry", "apple", "banana"}},
		{testName: "unknown orderBy falls back to created_at", orderBy: "bogus", expectedCarNames: []string{"cherry", "apple", "banana"}},
		{testName: "unknown sortIn falls back to asc", orderBy: "car_id", sortIn: "bogus", expectedCarNames: []string{"apple", "banana", "cherry"}},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			query := url.Values{}
			if test.orderBy != "" {
				query.Set("orderBy", test.orderBy)
			}
			if test.sortIn != "" {
				query.Set("sortIn", test.sortIn)
			}

			rec := executeRequest(t, app, http.MethodGet, "/orders?"+query.Encode(), token, nil)
			require.Equal(t, http.StatusOK, rec.Code)

			resp := decodeBody[handler.GetOrdersResponse](t, rec)
			names := make([]string, len(resp.Items))
			for i, item := range resp.Items {
				names[i] = item.CarName
				require.Equal(t, userNameByCarName[item.CarName], item.UserName)
			}
			require.Equal(t, test.expectedCarNames, names)
		})
	}
}

func TestGetOrders_WithFilteringByCarIDs(t *testing.T) {
	app, db, trustedKey := setupTestApp(t, nil)
	token := tokenFor(t, trustedKey, 1, auth.AdminRole)

	orders := seedOrders(t, db, []domain.Order{
		{CarID: 1, CarName: "apple", UserID: 1},
		{CarID: 2, CarName: "banana", UserID: 1},
		{CarID: 2, CarName: "banana", UserID: 1},
	})

	tests := []struct {
		testName      string
		carIDsParam   string
		expectedCode  int
		expectedOrder []int // indexes into orders
	}{
		{
			testName:      "one ID",
			carIDsParam:   "1",
			expectedCode:  http.StatusOK,
			expectedOrder: []int{0},
		},
		{
			testName:      "multiple IDs",
			carIDsParam:   "1,2",
			expectedCode:  http.StatusOK,
			expectedOrder: []int{0, 1, 2},
		},
		{
			testName:      "one ID and one non-existent ID",
			carIDsParam:   "1,3",
			expectedCode:  http.StatusOK,
			expectedOrder: []int{0},
		},
		{
			testName:      "non-existent ID only",
			carIDsParam:   "3",
			expectedCode:  http.StatusOK,
			expectedOrder: []int{},
		},
		{
			testName:      "empty car_ids param",
			carIDsParam:   "",
			expectedCode:  http.StatusOK,
			expectedOrder: []int{0, 1, 2},
		},
		{
			testName:    "non-numeric ID",
			carIDsParam: "abc", expectedCode: http.StatusBadRequest,
		},
		{
			testName:     "numeric and non-numeric IDs",
			carIDsParam:  "1,abc",
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			path := "/orders?car_ids=" + url.QueryEscape(test.carIDsParam)
			rec := executeRequest(t, app, http.MethodGet, path, token, nil)
			require.Equal(t, test.expectedCode, rec.Code)

			if test.expectedCode != http.StatusOK {
				return
			}

			resp := decodeBody[handler.GetOrdersResponse](t, rec)

			returnedIDs := make([]uint, len(resp.Items))
			for i, item := range resp.Items {
				returnedIDs[i] = item.ID
			}
			expectedIDs := make([]uint, len(test.expectedOrder))
			for i, idx := range test.expectedOrder {
				expectedIDs[i] = orders[idx].ID
			}
			require.ElementsMatch(t, expectedIDs, returnedIDs)
		})
	}
}

func TestGetOrders_WithPagination(t *testing.T) {
	app, db, trustedKey := setupTestApp(t, nil)
	token := tokenFor(t, trustedKey, 1, auth.AdminRole)
	orders := make([]domain.Order, 100)
	for i := range orders {
		orders[i] = domain.Order{CarID: 1, CarName: strconv.Itoa(i), UserID: 1}
	}
	seedOrders(t, db, orders)

	tests := []struct {
		name                                  string
		page, limit                           string
		expectedCode                          int
		expectedItemCount, expectedTotalCount int
		expectedFirstCarName                  string
		expectedHasNext                       bool
	}{
		{
			name: "blank page and blank limit",
			page: "", limit: "",
			expectedCode: http.StatusOK, expectedItemCount: 20, expectedTotalCount: 100, expectedFirstCarName: "0", expectedHasNext: true,
		},
		{
			name: "page and limit",
			page: "2", limit: "5",
			expectedCode: http.StatusOK, expectedItemCount: 5, expectedTotalCount: 100, expectedFirstCarName: "5", expectedHasNext: true,
		},
		{
			name: "blank page and limit",
			page: "", limit: "5",
			expectedCode: http.StatusOK, expectedItemCount: 5, expectedTotalCount: 100, expectedFirstCarName: "0", expectedHasNext: true,
		},
		{
			name: "page and blank limit",
			page: "2", limit: "",
			expectedCode: http.StatusOK, expectedItemCount: 20, expectedTotalCount: 100, expectedFirstCarName: "20", expectedHasNext: true,
		},
		{
			name: "last page has no next page",
			page: "5", limit: "20",
			expectedCode: http.StatusOK, expectedItemCount: 20, expectedTotalCount: 100, expectedFirstCarName: "80", expectedHasNext: false,
		},
		{
			name: "page past the last page is empty",
			page: "6", limit: "20",
			expectedCode: http.StatusOK, expectedItemCount: 0, expectedTotalCount: 100, expectedHasNext: false,
		},
		{
			name: "page offset exceeds total count",
			page: "2", limit: "101",
			expectedCode: http.StatusOK, expectedItemCount: 0, expectedTotalCount: 100, expectedHasNext: false,
		},
		{name: "non-numeric page", page: "abc", limit: "", expectedCode: http.StatusBadRequest},
		{name: "non-numeric limit", page: "", limit: "abc", expectedCode: http.StatusBadRequest},
		{name: "limit above the maximum", page: "", limit: strconv.Itoa(config.MaxPageLimit + 1), expectedCode: http.StatusBadRequest},
		{name: "page 0", page: "0", limit: "", expectedCode: http.StatusBadRequest},
		{name: "limit 0", page: "", limit: "0", expectedCode: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := fmt.Sprintf("/orders?page=%s&limit=%s&orderBy=created_at&sortIn=asc", test.page, test.limit)
			rec := executeRequest(t, app, http.MethodGet, path, token, nil)
			require.Equal(t, test.expectedCode, rec.Code)

			if test.expectedCode != http.StatusOK {
				return
			}

			resp := decodeBody[handler.GetOrdersResponse](t, rec)
			require.Len(t, resp.Items, test.expectedItemCount)
			require.Equal(t, test.expectedTotalCount, resp.Total)
			require.Equal(t, test.expectedHasNext, resp.HasNext)

			if test.expectedItemCount > 0 {
				require.Equal(t, test.expectedFirstCarName, resp.Items[0].CarName)
			}
		})
	}
}

func TestGetOrder_ById(t *testing.T) {
	const (
		ownerID         = uint(1)
		otherCustomerID = uint(2)
		adminID         = uint(99)
	)

	app, db, trustedKey := setupTestApp(t, nil)
	order := seedOrders(t, db, []domain.Order{
		{CarID: 5, CarName: "test car", UserName: "customer name", UserID: ownerID, PriceCents: 100},
	})[0]

	tests := []struct {
		name         string
		idString     string
		noToken      bool
		tokenUserID  uint
		tokenRole    auth.UserRole
		expectedCode int
	}{
		{name: "found by admin", idString: fmt.Sprint(order.ID), tokenUserID: adminID, tokenRole: auth.AdminRole, expectedCode: http.StatusOK},
		{name: "found by the owner", idString: fmt.Sprint(order.ID), tokenUserID: ownerID, tokenRole: auth.CustomerRole, expectedCode: http.StatusOK},
		{name: "forbidden for another customer", idString: fmt.Sprint(order.ID), tokenUserID: otherCustomerID, tokenRole: auth.CustomerRole, expectedCode: http.StatusForbidden},
		{name: "no token", idString: fmt.Sprint(order.ID), noToken: true, expectedCode: http.StatusUnauthorized},
		{name: "not found", idString: fmt.Sprint(order.ID + 1), tokenUserID: adminID, tokenRole: auth.AdminRole, expectedCode: http.StatusNotFound},
		{name: "non-numeric ID", idString: "abc", tokenUserID: adminID, tokenRole: auth.AdminRole, expectedCode: http.StatusBadRequest},
		{name: "ID 0", idString: "0", tokenUserID: adminID, tokenRole: auth.AdminRole, expectedCode: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var token string
			if !test.noToken {
				token = tokenFor(t, trustedKey, test.tokenUserID, test.tokenRole)
			}

			rec := executeRequest(t, app, http.MethodGet, "/orders/"+test.idString, token, nil)
			require.Equal(t, test.expectedCode, rec.Code)

			if test.expectedCode != http.StatusOK {
				return
			}

			resp := decodeBody[handler.GetOrderResponse](t, rec)
			require.Equal(t, order.ID, resp.Item.ID)
			require.Equal(t, order.CarName, resp.Item.CarName)
			require.Equal(t, order.UserName, resp.Item.UserName)
			require.Equal(t, order.PriceCents, resp.Item.PriceCents)
		})
	}
}

func TestUpdateOrder(t *testing.T) {
	const (
		originalName     = "original"
		originalCustomer = "original customer"
		originalPrice    = uint(100)
	)

	app, db, trustedKey := setupTestApp(t, nil)

	tests := []struct {
		testName     string
		payload      any
		idOffset     uint // 0 = the seeded order, 1 = an ID that doesn't exist
		noToken      bool
		role         auth.UserRole
		expectedCode int
		wantPrice    uint
		wantCarName  string
	}{
		{
			testName: "success - update price",
			payload:  map[string]any{"price_cents": 10},
			role:     auth.AdminRole, expectedCode: http.StatusOK,
			wantPrice: 10, wantCarName: originalName,
		},
		{
			testName: "success - update car name",
			payload:  map[string]any{"car_name": "renamed"},
			role:     auth.AdminRole, expectedCode: http.StatusOK,
			wantPrice: originalPrice, wantCarName: "renamed",
		},
		{
			testName: "success - empty payload changes nothing",
			payload:  map[string]any{},
			role:     auth.AdminRole, expectedCode: http.StatusOK,
			wantPrice: originalPrice, wantCarName: originalName,
		},
		{
			testName: "fail - non-existent ID",
			payload:  map[string]any{"price_cents": 10},
			idOffset: 1,
			role:     auth.AdminRole, expectedCode: http.StatusNotFound,
			wantPrice: originalPrice, wantCarName: originalName,
		},
		{
			testName: "fail - negative price_cents",
			payload:  map[string]any{"price_cents": -10},
			role:     auth.AdminRole, expectedCode: http.StatusBadRequest,
			wantPrice: originalPrice, wantCarName: originalName,
		},
		{
			testName: "fail - customer token",
			payload:  map[string]any{"price_cents": 10},
			role:     auth.CustomerRole, expectedCode: http.StatusForbidden,
			wantPrice: originalPrice, wantCarName: originalName,
		},
		{
			testName: "fail - no token",
			payload:  map[string]any{"price_cents": 10},
			noToken:  true, expectedCode: http.StatusUnauthorized,
			wantPrice: originalPrice, wantCarName: originalName,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			order := seedOrders(t, db, []domain.Order{
				{CarID: 5, CarName: originalName, UserID: 1, UserName: originalCustomer, PriceCents: originalPrice},
			})[0]

			var token string
			if !test.noToken {
				token = tokenFor(t, trustedKey, 1, test.role)
			}

			path := fmt.Sprintf("/orders/%d", order.ID+test.idOffset)
			rec := executeRequest(t, app, http.MethodPatch, path, token, test.payload)
			require.Equal(t, test.expectedCode, rec.Code)

			var updated domain.Order
			require.NoError(t, db.First(&updated, order.ID).Error)
			require.Equal(t, test.wantPrice, updated.PriceCents)
			require.Equal(t, test.wantCarName, updated.CarName)
			require.Equal(t, originalCustomer, updated.UserName)

			if test.expectedCode == http.StatusOK {
				resp := decodeBody[handler.UpdateOrderResponse](t, rec)
				require.Equal(t, order.ID, resp.Item.ID)
				require.Equal(t, updated.PriceCents, resp.Item.PriceCents)
				require.Equal(t, updated.CarName, resp.Item.CarName)
				require.Equal(t, originalCustomer, resp.Item.UserName)
			}
		})
	}
}

func TestDeleteOrder(t *testing.T) {
	app, db, trustedKey := setupTestApp(t, nil)

	tests := []struct {
		testName     string
		idOffset     uint // 0 = the seeded order, 1 = an ID that doesn't exist
		noToken      bool
		role         auth.UserRole
		expectedCode int
	}{
		{testName: "success", role: auth.AdminRole, expectedCode: http.StatusOK},
		{testName: "fail - non-existent ID", idOffset: 1, role: auth.AdminRole, expectedCode: http.StatusNotFound},
		{testName: "fail - no token", noToken: true, expectedCode: http.StatusUnauthorized},
		{testName: "fail - customer token", role: auth.CustomerRole, expectedCode: http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			order := seedOrders(t, db, []domain.Order{{CarID: 5, CarName: "test car", UserID: 1}})[0]

			var token string
			if !test.noToken {
				token = tokenFor(t, trustedKey, 1, test.role)
			}

			path := fmt.Sprintf("/orders/%d", order.ID+test.idOffset)
			rec := executeRequest(t, app, http.MethodDelete, path, token, nil)
			require.Equal(t, test.expectedCode, rec.Code)

			err := db.First(&domain.Order{}, order.ID).Error
			if test.expectedCode == http.StatusOK {
				require.ErrorIs(t, err, gorm.ErrRecordNotFound, "the order should have been deleted")
			} else {
				require.NoError(t, err, "a rejected request must not delete the order")
			}
		})
	}
}

func TestOrderRoutes_Authorization(t *testing.T) {
	app, _, trustedKey := setupTestApp(t, nil)
	attackerKey := newTestKey(t)

	type route struct{ method, path string }
	adminOnly := []route{
		{http.MethodGet, "/orders"},
		{http.MethodPatch, "/orders/1"},
		{http.MethodDelete, "/orders/1"},
	}
	anyRole := []route{
		{http.MethodPost, "/orders"},
		{http.MethodGet, "/orders/1"},
	}
	protected := append(append([]route{}, adminOnly...), anyRole...)

	invalidTokens := map[string]string{
		"no token":                    "",
		"expired token":               signRS256(t, trustedKey, 1, defaultTestUserName, auth.AdminRole, time.Now().Add(-time.Hour)),
		"token signed by another key": tokenFor(t, attackerKey, 1, auth.AdminRole),
	}

	for _, r := range protected {
		for name, token := range invalidTokens {
			t.Run(fmt.Sprintf("%s %s - %s", r.method, r.path, name), func(t *testing.T) {
				rec := executeRequest(t, app, r.method, r.path, token, nil)
				require.Equal(t, http.StatusUnauthorized, rec.Code)
			})
		}
	}

	for _, r := range adminOnly {
		t.Run(fmt.Sprintf("%s %s - customer token", r.method, r.path), func(t *testing.T) {
			token := tokenFor(t, trustedKey, 1, auth.CustomerRole)
			rec := executeRequest(t, app, r.method, r.path, token, nil)
			require.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
}
