package errors

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidPairKey = errors.New("invalid pair key")
)
