package database

import (
	"strings"
	"time"
)

// IsBusyOrLocked returns true if the error represents a SQLite database lock or busy error.
func IsBusyOrLocked(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "database is locked") ||
		strings.Contains(errMsg, "database table is locked") ||
		strings.Contains(errMsg, "sqlite_busy") ||
		strings.Contains(errMsg, "busy")
}

// ExecWithRetry runs the given database operation up to maxAttempts times if it encounters a SQLite busy or lock error.
func ExecWithRetry(maxAttempts int, fn func() error) error {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if !IsBusyOrLocked(err) {
			return err
		}
		if i < maxAttempts-1 {
			// Progressive backoff: 50ms, 100ms, 150ms, 200ms...
			backoff := time.Duration((i+1)*50) * time.Millisecond
			time.Sleep(backoff)
		}
	}
	return lastErr
}

// WithRetry executes a function returning a value and an error, retrying on SQLite busy/locked errors.
func WithRetry[T any](maxAttempts int, fn func() (T, error)) (T, error) {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	var zero T
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		val, err := fn()
		if err == nil {
			return val, nil
		}
		lastErr = err
		if !IsBusyOrLocked(err) {
			return zero, err
		}
		if i < maxAttempts-1 {
			backoff := time.Duration((i+1)*50) * time.Millisecond
			time.Sleep(backoff)
		}
	}
	return zero, lastErr
}
