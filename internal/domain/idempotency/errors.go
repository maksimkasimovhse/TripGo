package idempotency

import "errors"

var (
	ErrKeyNotFound            = errors.New("idempotency key not found")
	ErrIdempotencyKeyConflict = errors.New("idempotency key conflict")
	ErrRequestInProgress      = errors.New("request in progress")
)
