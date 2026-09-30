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
}
