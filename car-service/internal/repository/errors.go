package repository

import "errors"

var ErrOptimisticLockFailed = errors.New("optimistic lock conflict")

var ErrItemNotFound = errors.New("item not found")

var ErrItemNotAvailable = errors.New("item not available")

var ErrCarAlreadyExists = errors.New("car already exists")
