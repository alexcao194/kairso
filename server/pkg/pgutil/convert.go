package pgutil

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexcao194/kairso/pkg/vntime"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Text is the string, or "" for NULL.
func Text(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

// TextPtr is a pointer to the string, or nil for NULL.
func TextPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

// TextFrom is NULL for nil.
func TextFrom(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

// TextOrNull is NULL for "".
func TextOrNull(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

// TimePtr is a pointer to the time, or nil for NULL.
func TimePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

// Timestamptz is NULL for nil.
func Timestamptz(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *v, Valid: true}
}

// TimestamptzIf is NULL unless valid.
func TimestamptzIf(v time.Time, valid bool) pgtype.Timestamptz {
	if !valid {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: v, Valid: true}
}

// UUIDOrNull is NULL for a blank string, else the parsed UUID.
func UUIDOrNull(v string) (pgtype.UUID, error) {
	if strings.TrimSpace(v) == "" {
		return pgtype.UUID{}, nil
	}
	return ParseUUID(v)
}

// Int8OrNull is NULL for 0.
func Int8OrNull(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

// Int8Value is the number, or 0 for NULL.
func Int8Value(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

// DateString is YYYY-MM-DD, or "" for NULL.
func DateString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format(time.DateOnly)
}

// MonthRange is the first and last second of a YYYY-MM month in Vietnam; both NULL for "".
func MonthRange(month string) (from, to pgtype.Timestamptz, err error) {
	if month == "" {
		return from, to, nil
	}
	start, err := time.ParseInLocation("2006-01", month, vntime.Location)
	if err != nil {
		return from, to, fmt.Errorf("month phải theo định dạng YYYY-MM")
	}
	end := start.AddDate(0, 1, 0).Add(-time.Second)
	return pgtype.Timestamptz{Time: start, Valid: true}, pgtype.Timestamptz{Time: end, Valid: true}, nil
}

// Found turns pgx.ErrNoRows into ok=false without an error.
func Found[T any](v T, err error) (T, bool, error) {
	var zero T
	if errors.Is(err, pgx.ErrNoRows) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}
	return v, true, nil
}
