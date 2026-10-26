package businessevents

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestPostgresAtomicReplayAndConflict(t *testing.T) {
	dsn := os.Getenv("BUSINESS_EVENTS_TEST_DSN")
	if dsn == "" {
		t.Skip("set BUSINESS_EVENTS_TEST_DSN to a disposable migrated database")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close(ctx)) }()
	event := sample(t)
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "CREATE TEMP TABLE business_event_test_state (id integer PRIMARY KEY)")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO business_event_test_state VALUES (42)")
	require.NoError(t, err)
	inserted, err := Insert(ctx, tx, event)
	require.NoError(t, err)
	require.True(t, inserted)
	inserted, err = Insert(ctx, tx, event)
	require.NoError(t, err)
	require.False(t, inserted)
	event.Data["changed"] = true
	_, err = Insert(ctx, tx, event)
	require.ErrorIs(t, err, ErrConflict)
	require.NoError(t, tx.Rollback(ctx))
	var count int
	require.NoError(t, conn.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE id=$1::uuid", event.ID).Scan(&count))
	require.Zero(t, count)
	// A journal insert error aborts the state transaction too.
	tx, err = conn.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "CREATE TEMP TABLE business_event_test_state (id integer PRIMARY KEY)")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO business_event_test_state VALUES (42)")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO business_events(id,aggregatetype,aggregateid,type,payload,occurred_at) VALUES($1::uuid,'x','x:1','x','{}',now())", event.ID)
	require.Error(t, err)
	require.Error(t, tx.Commit(ctx))
	require.NoError(t, conn.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE id=$1::uuid", event.ID).Scan(&count))
	require.Zero(t, count)
}
