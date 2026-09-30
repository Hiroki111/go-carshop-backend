// Package carclienttest provides test doubles for carclient.CarClient. It
// mirrors the standard library's own convention for this situation (see
// net/http/httptest, io/fstest, testing/iotest): test-only helpers live in
// their own package so they can be imported from other packages' tests
// (like cmd/web's), while nothing outside test files ever imports
// carclienttest, so it never links into a production build.
package carclienttest

import (
	"context"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient"
)

// FakeCarClient is a test double implementing carclient.CarClient. Set
// GetCarByIDFunc to control what "car-service" returns for a given test
// case (an available car, an unavailable one, carclient.ErrCarNotFound, a
// transport error, etc.) without making a real HTTP call.
type FakeCarClient struct {
	GetCarByIDFunc func(ctx context.Context, id uint) (carclient.Car, error)
}

func (f *FakeCarClient) GetCarByID(ctx context.Context, id uint) (carclient.Car, error) {
	if f.GetCarByIDFunc != nil {
		return f.GetCarByIDFunc(ctx, id)
	}

	return carclient.Car{ID: id, Name: "Test Car", PriceCents: 100, IsAvailable: true}, nil
}
