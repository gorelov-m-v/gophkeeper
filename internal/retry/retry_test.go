package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func withoutRetryDelays(t *testing.T) {
	t.Helper()
	original := retryDelays
	retryDelays = []time.Duration{0, 0, 0}
	t.Cleanup(func() {
		retryDelays = original
	})
}

func TestIsRetriable(t *testing.T) {
	if IsRetriable(nil) {
		t.Fatal("IsRetriable(nil) = true, want false")
	}
	if IsRetriable(errors.New("plain error")) {
		t.Fatal("IsRetriable(plain) = true, want false")
	}
	if !IsRetriable(&pgconn.PgError{Code: pgerrcode.ConnectionException}) {
		t.Fatal("IsRetriable(connection exception) = false, want true")
	}
	if IsRetriable(&pgconn.PgError{Code: pgerrcode.UniqueViolation}) {
		t.Fatal("IsRetriable(unique violation) = true, want false")
	}
}

func TestDoSuccessAndNonRetriable(t *testing.T) {
	calls := 0
	if err := Do(func() error {
		calls++
		return nil
	}); err != nil {
		t.Fatalf("Do(success) error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("Do(success) calls = %d, want 1", calls)
	}

	wantErr := errors.New("validation failed")
	calls = 0
	err := Do(func() error {
		calls++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Do(non-retriable) error = %v, want %v", err, wantErr)
	}
	if calls != 1 {
		t.Fatalf("Do(non-retriable) calls = %d, want 1", calls)
	}
}

func TestDoRetriesConnectionErrors(t *testing.T) {
	withoutRetryDelays(t)

	calls := 0
	err := Do(func() error {
		calls++
		if calls < 2 {
			return &pgconn.PgError{Code: pgerrcode.ConnectionException}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do(retry then success) error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("Do(retry then success) calls = %d, want 2", calls)
	}
}

func TestDoReturnsLastRetriableError(t *testing.T) {
	withoutRetryDelays(t)

	calls := 0
	err := Do(func() error {
		calls++
		return &pgconn.PgError{Code: pgerrcode.ConnectionException}
	})
	if err == nil {
		t.Fatal("Do(always retriable) error = nil, want error")
	}
	if !IsRetriable(err) {
		t.Fatalf("Do(always retriable) error = %v, want retriable pg error", err)
	}
	if calls != 4 {
		t.Fatalf("Do(always retriable) calls = %d, want 4", calls)
	}
}
