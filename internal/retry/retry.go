// Package retry provides retry helpers for transient PostgreSQL failures.
package retry

import (
	"errors"
	"time"

	retrygo "github.com/avast/retry-go/v4"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

var retryDelays = []time.Duration{
	200 * time.Millisecond,
	500 * time.Millisecond,
	1 * time.Second,
}

// IsRetriable reports whether the given error should be retried.
func IsRetriable(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgerrcode.IsConnectionException(pgErr.Code)
}

// Do executes the given operation with retry semantics for transient PostgreSQL failures.
func Do(operation func() error) error {
	err := operation()
	if err == nil || !IsRetriable(err) {
		return err
	}

	return retrygo.Do(
		operation,
		retrygo.Attempts(uint(len(retryDelays))),
		retrygo.DelayType(func(n uint, err error, _ *retrygo.Config) time.Duration {
			if n == 0 {
				return retryDelays[0]
			}
			idx := int(n - 1)
			if idx >= len(retryDelays) {
				return retryDelays[len(retryDelays)-1]
			}
			return retryDelays[idx]
		}),
		retrygo.RetryIf(IsRetriable),
		retrygo.LastErrorOnly(true),
	)
}
