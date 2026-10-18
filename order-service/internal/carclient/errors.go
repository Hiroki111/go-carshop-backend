package carclient

import "errors"

var (
	ErrCarNotFound = errors.New("car not found")

	// ErrCarNotAvailable means car-service answered that the car cannot be
	// reserved for this order (reserved for another order, sold, withdrawn).
	// It is a definite answer, unlike a timeout.
	ErrCarNotAvailable = errors.New("car not available")
)
