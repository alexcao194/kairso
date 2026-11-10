package pgutil

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ParseUUID converts a string to a pgtype.UUID.
func ParseUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

// MustUUID converts a string to a pgtype.UUID, ignoring errors.
func MustUUID(id string) pgtype.UUID {
	parsed, _ := ParseUUID(id)
	return parsed
}

// UUIDToString converts a pgtype.UUID to a string representation.
func UUIDToString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}
