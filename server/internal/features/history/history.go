// Package history records field changes of workspaces, projects and modules.
package history

import (
	"context"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/alexcao194/kairso/pkg/ptr"
	"github.com/jackc/pgx/v5/pgtype"
)

// Insert writes one change of a field, unless the value did not change.
func Insert(ctx context.Context, q *database.Queries, entityType, entityID string, changedByID pgtype.UUID, field string, oldValue, newValue *string) error {
	if ptr.Equal(oldValue, newValue) {
		return nil
	}
	eid, err := pgutil.ParseUUID(entityID)
	if err != nil {
		return err
	}
	return q.InsertEntityHistory(ctx, database.InsertEntityHistoryParams{
		EntityType:  entityType,
		EntityID:    eid,
		ChangedByID: changedByID,
		Field:       field,
		OldValue:    pgutil.TextFrom(oldValue),
		NewValue:    pgutil.TextFrom(newValue),
	})
}
