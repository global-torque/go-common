package businessevents

import (
	"context"
	"errors"
	"fmt"

	"github.com/global-torque/go-common/queue/v2/pclient"
	"github.com/jackc/pgx/v5"
)

// QueryRowExecutor is the transaction's pgx query surface. Pass the actual open
// transaction, never a pool. Insert neither acquires a connection nor commits.
type QueryRowExecutor interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ErrConflict means an application UUID already names a different immutable fact.
var ErrConflict = errors.New("business event ID conflicts with existing fact")

// Insert appends a fact. An identical replay returns false; a content conflict
// must be propagated so the caller rolls back its entire business transaction.
func Insert(ctx context.Context, tx QueryRowExecutor, event pclient.Event) (bool, error) {
	payload, err := Marshal(event)
	if err != nil {
		return false, err
	}

	key := event.ObjectName + ":" + event.ObjectRef
	args := []any{event.ID, event.ObjectName, key, string(event.Action), string(payload), *event.OccurredAt}

	var id string

	err = tx.QueryRow(ctx, `INSERT INTO public.business_events
 (id,aggregatetype,aggregateid,type,payload,occurred_at) VALUES ($1::uuid,$2,$3,$4,$5::jsonb,$6)
 ON CONFLICT (id) DO NOTHING RETURNING id::text`, args...).Scan(&id)
	if err == nil {
		return true, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("insert business event: %w", err)
	}

	var equal bool

	err = tx.QueryRow(ctx, `SELECT aggregatetype=$2 AND aggregateid=$3 AND type=$4
 AND payload=$5::jsonb AND occurred_at=$6 FROM public.business_events WHERE id=$1::uuid`, args...).Scan(&equal)
	if err != nil {
		return false, fmt.Errorf("compare business event replay: %w", err)
	}

	if !equal {
		return false, ErrConflict
	}

	return false, nil
}
