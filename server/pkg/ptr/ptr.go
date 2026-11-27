// Package ptr has small helpers for optional (pointer) values.
package ptr

import "time"

// Value is *p, or the zero value for nil.
func Value[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// Equal reports whether both are nil or both point to equal values.
func Equal[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// NonEmpty is &s, or nil for "".
func NonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// RFC3339 formats a time in UTC (RFC 3339 with nanoseconds), or nil.
func RFC3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339Nano)
	return &s
}
