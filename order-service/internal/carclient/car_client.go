package carclient

import (
	"context"
)

// CarClient is the abstraction Service depends on instead of the concrete
// HTTPCarClient. Declaring it here, alongside the implementations, lets
// *HTTPCarClient (the real, production implementation, built by
// NewHTTPCarClient) and a test double (see the carclienttest subpackage)
// both satisfy it - neither one needs to import the other or even know this
// interface exists - so tests can substitute a fake without making a real
// HTTP call.
type CarClient interface {
	GetCarByID(ctx context.Context, id uint) (Car, error)

	// ReserveCar asks car-service to reserve the car for the order. The order
	// ID is the idempotency key: repeating the call for the same order is
	// safe. It returns ErrCarNotFound or ErrCarNotAvailable for definite
	// answers; any other error means the outcome is unknown.
	ReserveCar(ctx context.Context, carID, orderID uint) (Car, error)

	// ReleaseCar asks car-service to put the car this order reserved back on
	// sale. The order ID is the idempotency key: repeating the call for the
	// same order is safe. It returns ErrCarNotFound or ErrCarNotAvailable for
	// definite answers; any other error means the outcome is unknown.
	ReleaseCar(ctx context.Context, carID, orderID uint) error
}

// TokenSource hands out the bearer token this service presents to
// car-service's internal endpoints. serviceauth.ClientCredentials is the
// production implementation.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}
