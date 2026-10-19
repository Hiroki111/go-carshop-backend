package repository

import "errors"

// TODO: Use this for updating an order
var ErrOptimisticLockFailed = errors.New("optimistic lock conflict")

var (
	ErrItemNotFound        = errors.New("item not found")
	ErrItemNotAvailable    = errors.New("item not available")
	ErrOrderStatusConflict = errors.New("order is not in the expected status")
)
