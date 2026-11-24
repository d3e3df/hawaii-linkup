package errors

import "errors"

var (
	ErrNotFound             = errors.New("not found")
	ErrTargetNotFound       = errors.New("target not found")
	ErrInvalidPairKey       = errors.New("invalid pair key")
	ErrDatabaseTimeout      = errors.New("database timeout")
	ErrTransactionRetryable = errors.New("transaction retryable")
	ErrCommitUnconfirmed    = errors.New("commit unconfirmed")
)
