package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient/carclienttest"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/handler"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	eventTypeOrderConfirmed = "order.confirmed"
	eventTypeOrderCancelled = "order.cancelled"

	eventIDPattern = `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
)

// orderEventPayload is the contract with message-service. It is declared here
// with the JSON field names spelled out, rather than reusing the type the
// service marshals, so that renaming a field breaks these tests.
type orderEventPayload struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
	OrderID    uint      `json:"order_id"`
	UserID     uint      `json:"user_id"`
	CarID      uint      `json:"car_id"`
	CarName    string    `json:"car_name"`
	PriceCents uint      `json:"price_cents"`
}

// outboxEvents returns the recorded events in the order they were recorded,
// which is the order the relay will publish them in.
func outboxEvents(t *testing.T, db *gorm.DB) []domain.OutboxEvent {
	t.Helper()

	var events []domain.OutboxEvent
	require.NoError(t, db.Order("id").Find(&events).Error)
	return events
}

func eventTypesOf(events []domain.OutboxEvent) []domain.EventType {
	types := make([]domain.EventType, len(events))
	for i, event := range events {
		types[i] = event.EventType
	}
	return types
}

func decodeEventPayload(t *testing.T, event domain.OutboxEvent) orderEventPayload {
	t.Helper()

	var payload orderEventPayload
	require.NoError(t, json.Unmarshal([]byte(event.Payload), &payload))
	return payload
}

// requireOrderEvent checks one recorded event against the order it describes:
// the columns the relay works with, and the payload message-service receives.
// The payload must hold exactly the agreed fields. In particular it must not
// carry the customer's name or email.
func requireOrderEvent(t *testing.T, event domain.OutboxEvent, eventType domain.EventType, order domain.Order, notBefore, notAfter time.Time) {
	t.Helper()

	require.Equal(t, eventType, event.EventType)
	require.Equal(t, order.ID, event.OrderID, "the relay uses the order ID as the Kafka key")
	require.Regexp(t, eventIDPattern, event.EventID)
	require.Nil(t, event.PublishedAt, "a new event is waiting to be published")

	payload := decodeEventPayload(t, event)
	require.False(t, payload.OccurredAt.Before(notBefore) || payload.OccurredAt.After(notAfter),
		"occurred_at %s is outside %s .. %s", payload.OccurredAt, notBefore, notAfter)

	require.Equal(t, event.EventID, payload.EventID)
	require.Equal(t, string(eventType), payload.EventType)
	require.Equal(t, 1, payload.Version)
	require.Equal(t, order.ID, payload.OrderID)
	require.Equal(t, order.UserID, payload.UserID)
	require.Equal(t, order.CarID, payload.CarID)
	require.Equal(t, order.CarName, payload.CarName)
	require.Equal(t, order.PriceCents, payload.PriceCents)

	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(event.Payload), &raw))
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, []string{
		"event_id", "event_type", "version", "occurred_at",
		"order_id", "user_id", "car_id", "car_name", "price_cents",
	}, keys)
}

func cancelPath(orderID uint) string {
	return fmt.Sprintf("/orders/%d/cancel", orderID)
}

func TestCreateOrder_RecordsAnOrderConfirmedEvent(t *testing.T) {
	const (
		carID  = uint(5)
		userID = uint(7)
	)

	carClient := &carclienttest.FakeCarClient{
		GetCarByIDFunc: func(ctx context.Context, id uint) (carclient.Car, error) {
			return carclient.Car{ID: id, Name: "Skoda Octavia", PriceCents: 1234500, Status: carclient.StatusAvailable}, nil
		},
	}
	app, db, trustedKey := setupTestApp(t, carClient)

	before := time.Now().Truncate(time.Second)
	rec := executeRequest(t, app, http.MethodPost, "/orders", tokenFor(t, trustedKey, userID, auth.CustomerRole), handler.CreateOrderRequest{CarID: carID})
	require.Equal(t, http.StatusCreated, rec.Code)
	after := time.Now()

	var order domain.Order
	require.NoError(t, db.First(&order, decodeBody[handler.CreateOrderResponse](t, rec).ID).Error)
	require.Equal(t, domain.OrderStatusConfirmed, order.Status)

	events := outboxEvents(t, db)
	require.Len(t, events, 1)
	requireOrderEvent(t, events[0], eventTypeOrderConfirmed, order, before, after)
}

func TestCancelOrder_RecordsAnOrderCancelledEvent(t *testing.T) {
	tests := []struct {
		testName    string
		orderStatus domain.OrderStatus
	}{
		{testName: "pending order", orderStatus: domain.OrderStatusPending},
		{testName: "confirmed order", orderStatus: domain.OrderStatusConfirmed},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			app, db, trustedKey := setupTestApp(t, nil)

			order := seedOrders(t, db, []domain.Order{
				{CarID: 5, CarName: "Skoda Octavia", UserID: 7, PriceCents: 1234500, Status: test.orderStatus},
			})[0]

			before := time.Now().Truncate(time.Second)
			rec := executeRequest(t, app, http.MethodPost, cancelPath(order.ID), tokenFor(t, trustedKey, 99, auth.AdminRole), nil)
			require.Equal(t, http.StatusOK, rec.Code)
			after := time.Now()

			events := outboxEvents(t, db)
			require.Len(t, events, 1)
			requireOrderEvent(t, events[0], eventTypeOrderCancelled, order, before, after)
		})
	}
}

func TestOrderLifecycle_EventsAreRecordedInOrder(t *testing.T) {
	app, db, trustedKey := setupTestApp(t, nil)

	placed := executeRequest(t, app, http.MethodPost, "/orders", tokenFor(t, trustedKey, 1, auth.CustomerRole), handler.CreateOrderRequest{CarID: 5})
	require.Equal(t, http.StatusCreated, placed.Code)
	orderID := decodeBody[handler.CreateOrderResponse](t, placed).ID

	cancelled := executeRequest(t, app, http.MethodPost, cancelPath(orderID), tokenFor(t, trustedKey, 99, auth.AdminRole), nil)
	require.Equal(t, http.StatusOK, cancelled.Code)

	events := outboxEvents(t, db)
	require.Equal(t, []domain.EventType{eventTypeOrderConfirmed, eventTypeOrderCancelled}, eventTypesOf(events))
	require.Equal(t, orderID, events[0].OrderID)
	require.Equal(t, orderID, events[1].OrderID)
	require.NotEqual(t, events[0].EventID, events[1].EventID, "every event has its own ID")
}

func TestCancelOrder_RepeatedCancellationRecordsOneEvent(t *testing.T) {
	app, db, trustedKey := setupTestApp(t, nil)
	admin := tokenFor(t, trustedKey, 99, auth.AdminRole)
	order := seedOrders(t, db, []domain.Order{
		{CarID: 5, CarName: "test car", UserID: 1, PriceCents: 100, Status: domain.OrderStatusConfirmed},
	})[0]

	first := executeRequest(t, app, http.MethodPost, cancelPath(order.ID), admin, nil)
	second := executeRequest(t, app, http.MethodPost, cancelPath(order.ID), admin, nil)

	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, []domain.EventType{eventTypeOrderCancelled}, eventTypesOf(outboxEvents(t, db)))
}

func TestCreateOrder_RetryOfAConfirmedOrderRecordsNoSecondEvent(t *testing.T) {
	app, db, trustedKey := setupTestApp(t, nil)
	customer := tokenFor(t, trustedKey, 1, auth.CustomerRole)
	body := handler.CreateOrderRequest{CarID: 5}

	first := executeRequest(t, app, http.MethodPost, "/orders", customer, body)
	retry := executeRequest(t, app, http.MethodPost, "/orders", customer, body)

	require.Equal(t, http.StatusCreated, first.Code)
	require.Equal(t, http.StatusOK, retry.Code)
	require.Equal(t, []domain.EventType{eventTypeOrderConfirmed}, eventTypesOf(outboxEvents(t, db)))
}

func TestCreateOrder_RetryOfAPendingOrderRecordsTheEventOnceItIsConfirmed(t *testing.T) {
	var reserveErr error
	carClient := &carclienttest.FakeCarClient{
		ReserveCarFunc: func(ctx context.Context, id, orderID uint) (carclient.Car, error) {
			return carclient.Car{ID: id, Status: carclient.StatusReserved}, reserveErr
		},
	}
	app, db, trustedKey := setupTestApp(t, carClient)
	customer := tokenFor(t, trustedKey, 1, auth.CustomerRole)
	body := handler.CreateOrderRequest{CarID: 5}

	reserveErr = errors.New("timeout")
	stuck := executeRequest(t, app, http.MethodPost, "/orders", customer, body)
	require.Equal(t, http.StatusServiceUnavailable, stuck.Code)
	require.Empty(t, outboxEvents(t, db), "a pending order is not announced")

	reserveErr = nil
	recovered := executeRequest(t, app, http.MethodPost, "/orders", customer, body)
	require.Equal(t, http.StatusOK, recovered.Code)
	require.Equal(t, []domain.EventType{eventTypeOrderConfirmed}, eventTypesOf(outboxEvents(t, db)))
}

func TestCreateOrder_RecordsNoEventWhenTheOrderIsNotConfirmed(t *testing.T) {
	tests := []struct {
		testName   string
		carStatus  string
		getCarErr  error
		reserveErr error

		expectedCode int
	}{
		{testName: "car-service has no such car", getCarErr: carclient.ErrCarNotFound, expectedCode: http.StatusNotFound},
		{testName: "car is already reserved", carStatus: carclient.StatusReserved, expectedCode: http.StatusConflict},
		{testName: "car is taken before it can be reserved", reserveErr: carclient.ErrCarNotAvailable, expectedCode: http.StatusConflict},
		{testName: "car disappears before it can be reserved", reserveErr: carclient.ErrCarNotFound, expectedCode: http.StatusNotFound},
		{testName: "reserving times out", reserveErr: context.DeadlineExceeded, expectedCode: http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			carClient := &carclienttest.FakeCarClient{
				GetCarByIDFunc: func(ctx context.Context, id uint) (carclient.Car, error) {
					status := test.carStatus
					if status == "" {
						status = carclient.StatusAvailable
					}
					return carclient.Car{ID: id, Name: "Test Car", PriceCents: 100, Status: status}, test.getCarErr
				},
				ReserveCarFunc: func(ctx context.Context, id, orderID uint) (carclient.Car, error) {
					return carclient.Car{ID: id, Status: carclient.StatusReserved}, test.reserveErr
				},
			}
			app, db, trustedKey := setupTestApp(t, carClient)

			rec := executeRequest(t, app, http.MethodPost, "/orders", tokenFor(t, trustedKey, 1, auth.CustomerRole), handler.CreateOrderRequest{CarID: 5})

			require.Equal(t, test.expectedCode, rec.Code)
			require.Empty(t, outboxEvents(t, db))
		})
	}
}

func TestCancelOrder_RecordsNoEventWhenTheOrderIsNotCancelled(t *testing.T) {
	tests := []struct {
		testName    string
		orderStatus domain.OrderStatus
		idOffset    uint
		noToken     bool
		role        auth.UserRole
		releaseErr  error

		expectedCode int
	}{
		{testName: "no token", orderStatus: domain.OrderStatusConfirmed, noToken: true, expectedCode: http.StatusUnauthorized},
		{testName: "customer token", orderStatus: domain.OrderStatusConfirmed, role: auth.CustomerRole, expectedCode: http.StatusForbidden},
		{testName: "non-existent order", orderStatus: domain.OrderStatusConfirmed, idOffset: 1, role: auth.AdminRole, expectedCode: http.StatusNotFound},
		{testName: "failed order", orderStatus: domain.OrderStatusFailed, role: auth.AdminRole, expectedCode: http.StatusConflict},
		{testName: "car-service refuses the release", orderStatus: domain.OrderStatusConfirmed, role: auth.AdminRole, releaseErr: carclient.ErrCarNotAvailable, expectedCode: http.StatusConflict},
		{testName: "car-service times out", orderStatus: domain.OrderStatusConfirmed, role: auth.AdminRole, releaseErr: context.DeadlineExceeded, expectedCode: http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			carClient := &carclienttest.FakeCarClient{
				ReleaseCarFunc: func(ctx context.Context, carID, orderID uint) error { return test.releaseErr },
			}
			app, db, trustedKey := setupTestApp(t, carClient)
			order := seedOrders(t, db, []domain.Order{
				{CarID: 5, CarName: "test car", UserID: 1, PriceCents: 100, Status: test.orderStatus},
			})[0]

			var token string
			if !test.noToken {
				token = tokenFor(t, trustedKey, 99, test.role)
			}
			rec := executeRequest(t, app, http.MethodPost, cancelPath(order.ID+test.idOffset), token, nil)

			require.Equal(t, test.expectedCode, rec.Code)
			require.Empty(t, outboxEvents(t, db))
		})
	}
}

func TestCreateOrder_OrderStaysPendingWhenTheEventCannotBeRecorded(t *testing.T) {
	app, db, trustedKey := setupTestApp(t, nil)
	customer := tokenFor(t, trustedKey, 1, auth.CustomerRole)
	body := handler.CreateOrderRequest{CarID: 5}
	require.NoError(t, db.Migrator().DropTable(&domain.OutboxEvent{}))

	failed := executeRequest(t, app, http.MethodPost, "/orders", customer, body)

	require.Equal(t, http.StatusServiceUnavailable, failed.Code)
	var order domain.Order
	require.NoError(t, db.First(&order).Error)
	require.Equal(t, domain.OrderStatusPending, order.Status)

	require.NoError(t, db.AutoMigrate(&domain.OutboxEvent{}))
	retry := executeRequest(t, app, http.MethodPost, "/orders", customer, body)

	require.Equal(t, http.StatusOK, retry.Code)
	require.Equal(t, domain.OrderStatusConfirmed, orderStatusInDB(t, db, order.ID))
	require.Equal(t, []domain.EventType{eventTypeOrderConfirmed}, eventTypesOf(outboxEvents(t, db)))
}

func TestCancelOrder_OrderIsNotCancelledWhenTheEventCannotBeRecorded(t *testing.T) {
	var releaseCalls []releaseCall
	carClient := &carclienttest.FakeCarClient{
		ReleaseCarFunc: func(ctx context.Context, carID, orderID uint) error {
			releaseCalls = append(releaseCalls, releaseCall{carID: carID, orderID: orderID})
			return nil
		},
	}
	app, db, trustedKey := setupTestApp(t, carClient)
	admin := tokenFor(t, trustedKey, 99, auth.AdminRole)
	order := seedOrders(t, db, []domain.Order{
		{CarID: 5, CarName: "test car", UserID: 1, PriceCents: 100, Status: domain.OrderStatusConfirmed},
	})[0]
	require.NoError(t, db.Migrator().DropTable(&domain.OutboxEvent{}))

	failed := executeRequest(t, app, http.MethodPost, cancelPath(order.ID), admin, nil)

	require.Equal(t, http.StatusInternalServerError, failed.Code)
	require.Equal(t, domain.OrderStatusConfirmed, orderStatusInDB(t, db, order.ID))

	require.NoError(t, db.AutoMigrate(&domain.OutboxEvent{}))
	retry := executeRequest(t, app, http.MethodPost, cancelPath(order.ID), admin, nil)

	require.Equal(t, http.StatusOK, retry.Code)
	require.Equal(t, domain.OrderStatusCancelled, orderStatusInDB(t, db, order.ID))
	require.Equal(t, []domain.EventType{eventTypeOrderCancelled}, eventTypesOf(outboxEvents(t, db)))
	call := releaseCall{carID: 5, orderID: order.ID}
	require.Equal(t, []releaseCall{call, call}, releaseCalls)
}
