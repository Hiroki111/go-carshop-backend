package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient/carclienttest"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/handler"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type releaseCall struct {
	carID, orderID uint
}

func orderStatusInDB(t *testing.T, db *gorm.DB, orderID uint) domain.OrderStatus {
	t.Helper()

	var order domain.Order
	require.NoError(t, db.First(&order, orderID).Error)
	return order.Status
}

func TestCancelOrder(t *testing.T) {
	const carID = uint(5)

	tests := []struct {
		testName    string
		orderStatus domain.OrderStatus
		idOffset    uint
		rawID       string
		noToken     bool
		role        auth.UserRole
		releaseErr  error

		expectedCode        int
		expectedOrderStatus domain.OrderStatus
		expectRelease       bool
	}{
		{
			testName:            "success - pending order",
			orderStatus:         domain.OrderStatusPending,
			role:                auth.AdminRole,
			expectedCode:        http.StatusOK,
			expectedOrderStatus: domain.OrderStatusCancelled,
			expectRelease:       true,
		},
		{
			testName:            "success - confirmed order",
			orderStatus:         domain.OrderStatusConfirmed,
			role:                auth.AdminRole,
			expectedCode:        http.StatusOK,
			expectedOrderStatus: domain.OrderStatusCancelled,
			expectRelease:       true,
		},
		{
			testName:            "success - already cancelled order",
			orderStatus:         domain.OrderStatusCancelled,
			role:                auth.AdminRole,
			expectedCode:        http.StatusOK,
			expectedOrderStatus: domain.OrderStatusCancelled,
		},
		{
			// A car that no longer exists can't stay reserved, so there is
			// nothing to release and the order can be cancelled.
			testName:            "success - car-service no longer has the car",
			orderStatus:         domain.OrderStatusPending,
			role:                auth.AdminRole,
			releaseErr:          carclient.ErrCarNotFound,
			expectedCode:        http.StatusOK,
			expectedOrderStatus: domain.OrderStatusCancelled,
			expectRelease:       true,
		},
		{
			testName:            "fail - no token",
			orderStatus:         domain.OrderStatusPending,
			noToken:             true,
			expectedCode:        http.StatusUnauthorized,
			expectedOrderStatus: domain.OrderStatusPending,
		},
		{
			testName:            "fail - customer token",
			orderStatus:         domain.OrderStatusPending,
			role:                auth.CustomerRole,
			expectedCode:        http.StatusForbidden,
			expectedOrderStatus: domain.OrderStatusPending,
		},
		{
			testName:            "fail - token without a role",
			orderStatus:         domain.OrderStatusPending,
			role:                "",
			expectedCode:        http.StatusForbidden,
			expectedOrderStatus: domain.OrderStatusPending,
		},
		{
			testName:            "fail - non-existent order",
			orderStatus:         domain.OrderStatusPending,
			idOffset:            1,
			role:                auth.AdminRole,
			expectedCode:        http.StatusNotFound,
			expectedOrderStatus: domain.OrderStatusPending,
		},
		{
			testName:            "fail - malformed order ID",
			orderStatus:         domain.OrderStatusPending,
			rawID:               "abc",
			role:                auth.AdminRole,
			expectedCode:        http.StatusBadRequest,
			expectedOrderStatus: domain.OrderStatusPending,
		},
		{
			testName:            "fail - failed order",
			orderStatus:         domain.OrderStatusFailed,
			role:                auth.AdminRole,
			expectedCode:        http.StatusConflict,
			expectedOrderStatus: domain.OrderStatusFailed,
		},
		{
			testName:            "fail - car-service times out",
			orderStatus:         domain.OrderStatusConfirmed,
			role:                auth.AdminRole,
			releaseErr:          context.DeadlineExceeded,
			expectedCode:        http.StatusServiceUnavailable,
			expectedOrderStatus: domain.OrderStatusConfirmed,
			expectRelease:       true,
		},
		{
			testName:            "fail - car-service returns an unexpected error",
			orderStatus:         domain.OrderStatusPending,
			role:                auth.AdminRole,
			releaseErr:          errors.New("car-service answered with an unexpected status"),
			expectedCode:        http.StatusServiceUnavailable,
			expectedOrderStatus: domain.OrderStatusPending,
			expectRelease:       true,
		},
		{
			testName:            "fail - car-service says the car can't be released",
			orderStatus:         domain.OrderStatusConfirmed,
			role:                auth.AdminRole,
			releaseErr:          carclient.ErrCarNotAvailable,
			expectedCode:        http.StatusConflict,
			expectedOrderStatus: domain.OrderStatusConfirmed,
			expectRelease:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			var releaseCalls []releaseCall
			carClient := &carclienttest.FakeCarClient{
				ReleaseCarFunc: func(ctx context.Context, gotCarID, gotOrderID uint) error {
					releaseCalls = append(releaseCalls, releaseCall{carID: gotCarID, orderID: gotOrderID})
					return test.releaseErr
				},
			}
			app, db, trustedKey := setupTestApp(t, carClient)
			order := seedOrders(t, db, []domain.Order{
				{CarID: carID, CarName: "test car", UserID: 1, Status: test.orderStatus},
			})[0]

			var token string
			if !test.noToken {
				token = tokenFor(t, trustedKey, 1, test.role)
			}

			id := strconv.FormatUint(uint64(order.ID+test.idOffset), 10)
			if test.rawID != "" {
				id = test.rawID
			}
			rec := executeRequest(t, app, http.MethodPost, "/orders/"+id+"/cancel", token, nil)

			require.Equal(t, test.expectedCode, rec.Code)
			require.Equal(t, test.expectedOrderStatus, orderStatusInDB(t, db, order.ID))
			if test.expectRelease {
				require.Equal(t, []releaseCall{{carID: carID, orderID: order.ID}}, releaseCalls)
			} else {
				require.Empty(t, releaseCalls, "car-service must not be asked to release anything")
			}
		})
	}
}

func TestCancelOrder_FreesTheCarForAnotherCustomer(t *testing.T) {
	const carID = uint(5)

	app, _, trustedKey := setupTestApp(t, nil)
	owner := tokenFor(t, trustedKey, 1, auth.CustomerRole)
	other := tokenFor(t, trustedKey, 2, auth.CustomerRole)
	admin := tokenFor(t, trustedKey, 99, auth.AdminRole)
	body := handler.CreateOrderRequest{CarID: carID}

	placed := executeRequest(t, app, http.MethodPost, "/orders", owner, body)
	require.Equal(t, http.StatusCreated, placed.Code)
	orderID := decodeBody[handler.CreateOrderResponse](t, placed).ID

	blocked := executeRequest(t, app, http.MethodPost, "/orders", other, body)
	require.Equal(t, http.StatusConflict, blocked.Code, "the confirmed order still holds the car")

	cancelled := executeRequest(t, app, http.MethodPost, fmt.Sprintf("/orders/%d/cancel", orderID), admin, nil)
	require.Equal(t, http.StatusOK, cancelled.Code)

	retry := executeRequest(t, app, http.MethodPost, "/orders", other, body)
	require.Equal(t, http.StatusCreated, retry.Code)
}

func TestCancelOrder_RetryAfterCarServiceFailureCompletesTheCancellation(t *testing.T) {
	const carID = uint(5)

	var releaseErr error
	var releaseCalls []releaseCall
	carClient := &carclienttest.FakeCarClient{
		ReleaseCarFunc: func(ctx context.Context, gotCarID, gotOrderID uint) error {
			releaseCalls = append(releaseCalls, releaseCall{carID: gotCarID, orderID: gotOrderID})
			return releaseErr
		},
	}
	app, db, trustedKey := setupTestApp(t, carClient)
	admin := tokenFor(t, trustedKey, 99, auth.AdminRole)
	order := seedOrders(t, db, []domain.Order{
		{CarID: carID, CarName: "test car", UserID: 1, Status: domain.OrderStatusConfirmed},
	})[0]
	path := fmt.Sprintf("/orders/%d/cancel", order.ID)

	releaseErr = context.DeadlineExceeded
	first := executeRequest(t, app, http.MethodPost, path, admin, nil)
	require.Equal(t, http.StatusServiceUnavailable, first.Code)
	require.Equal(t, domain.OrderStatusConfirmed, orderStatusInDB(t, db, order.ID))

	releaseErr = nil
	second := executeRequest(t, app, http.MethodPost, path, admin, nil)
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, domain.OrderStatusCancelled, orderStatusInDB(t, db, order.ID))

	call := releaseCall{carID: carID, orderID: order.ID}
	require.Equal(t, []releaseCall{call, call}, releaseCalls)
}
