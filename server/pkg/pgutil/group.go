package pgutil

import "github.com/jackc/pgx/v5/pgtype"

// IDs collects the UUID of each item, for "= ANY($1::uuid[])" batch queries.
func IDs[T any](items []T, id func(T) pgtype.UUID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(items))
	for _, it := range items {
		out = append(out, id(it))
	}
	return out
}

// GroupBy groups rows by a UUID column, keeping their order, so a batch query can stand in
// for one query per parent row.
func GroupBy[T any](rows []T, key func(T) pgtype.UUID) map[[16]byte][]T {
	out := make(map[[16]byte][]T)
	for _, r := range rows {
		k := key(r).Bytes
		out[k] = append(out[k], r)
	}
	return out
}
