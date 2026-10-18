package carclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTokenSource struct {
	token string
	err   error
	calls int
}

func (f *fakeTokenSource) Token(ctx context.Context) (string, error) {
	f.calls++
	return f.token, f.err
}

func TestReserveCar_SendsTheServiceTokenAndTheOrderID(t *testing.T) {
	type seenRequest struct {
		method, path, auth, contentType string
		body                            map[string]any
	}
	seen := make(chan seenRequest, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		seen <- seenRequest{
			method:      r.Method,
			path:        r.URL.Path,
			auth:        r.Header.Get("Authorization"),
			contentType: r.Header.Get("Content-Type"),
			body:        body,
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"item":{"id":5,"name":"Golf","price_cents":1200,"status":"reserved"}}`))
	}))
	defer srv.Close()

	tokens := &fakeTokenSource{token: "service-token"}
	client := carclient.NewHTTPCarClient(srv.URL, tokens)

	car, err := client.ReserveCar(context.Background(), 5, 42)
	require.NoError(t, err)

	got := <-seen
	require.Equal(t, http.MethodPost, got.method)
	require.Equal(t, "/internal/cars/5/reservations", got.path)
	require.Equal(t, "Bearer service-token", got.auth)
	require.Equal(t, "application/json", got.contentType)
	require.Equal(t, map[string]any{"order_id": float64(42)}, got.body)

	require.Equal(t, carclient.Car{ID: 5, Name: "Golf", PriceCents: 1200, Status: carclient.StatusReserved}, car)
}

func TestReserveCar_MapsCarServiceAnswers(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		expectedErr error // nil means "some other error"
	}{
		{name: "not found is a definite answer", status: http.StatusNotFound, expectedErr: carclient.ErrCarNotFound},
		{name: "conflict is a definite answer", status: http.StatusConflict, expectedErr: carclient.ErrCarNotAvailable},
		{name: "unauthorized is not a statement about the car", status: http.StatusUnauthorized},
		{name: "forbidden is not a statement about the car", status: http.StatusForbidden},
		{name: "bad request is not a statement about the car", status: http.StatusBadRequest},
		{name: "server error is not a statement about the car", status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
			}))
			defer srv.Close()

			client := carclient.NewHTTPCarClient(srv.URL, &fakeTokenSource{token: "t"})
			_, err := client.ReserveCar(context.Background(), 5, 42)

			require.Error(t, err)
			if test.expectedErr != nil {
				require.ErrorIs(t, err, test.expectedErr)
				return
			}
			require.False(t, errors.Is(err, carclient.ErrCarNotFound), "must not be mistaken for a definite answer")
			require.False(t, errors.Is(err, carclient.ErrCarNotAvailable), "must not be mistaken for a definite answer")
		})
	}
}

func TestReserveCar_WithoutAServiceTokenNothingIsSent(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()

	tokenErr := errors.New("user-service is down")
	client := carclient.NewHTTPCarClient(srv.URL, &fakeTokenSource{err: tokenErr})

	_, err := client.ReserveCar(context.Background(), 5, 42)

	require.ErrorIs(t, err, tokenErr)
	require.False(t, errors.Is(err, carclient.ErrCarNotFound))
	require.False(t, errors.Is(err, carclient.ErrCarNotAvailable))
	require.Zero(t, requests)
}
