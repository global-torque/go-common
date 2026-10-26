package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestScopeGuards(t *testing.T) {
	for _, test := range []struct {
		name      string
		closed    bool
		wrongPool bool
		want      error
	}{
		{name: "closed", closed: true, want: pgx.ErrTxClosed},
		{name: "wrong pool", wrongPool: true, want: ErrScopePool},
	} {
		t.Run(test.name, func(t *testing.T) {
			database := &DB{}
			scope := &transactionScope{}
			scope.closed.Store(test.closed)
			if test.wrongPool {
				scope.pool = &pgxpool.Pool{}
			}
			ctx := context.WithValue(context.Background(), scopeKey{}, scope)
			_, err := database.Exec(ctx, "select 1")
			require.ErrorIs(t, err, test.want)
			_, err = database.Query(ctx, "select 1")
			require.ErrorIs(t, err, test.want)
			require.ErrorIs(t, database.QueryRow(ctx, "select 1").Scan(), test.want)
			batch := database.SendBatch(ctx, &pgx.Batch{})
			_, err = batch.Exec()
			require.ErrorIs(t, err, test.want)
			_, err = batch.Query()
			require.ErrorIs(t, err, test.want)
			require.ErrorIs(t, batch.QueryRow().Scan(), test.want)
			require.ErrorIs(t, batch.Close(), test.want)
			_, err = database.CopyFrom(ctx, nil, nil, nil)
			require.ErrorIs(t, err, test.want)
			_, err = database.Begin(ctx)
			require.ErrorIs(t, err, test.want)
			_, err = database.BeginTx(ctx, pgx.TxOptions{})
			require.ErrorIs(t, err, test.want)
			_, _, err = database.BeginScope(ctx, pgx.TxOptions{})
			require.ErrorIs(t, err, test.want)
			_, err = database.Acquire(ctx)
			require.ErrorIs(t, err, test.want)
			require.ErrorIs(t, database.AcquireFunc(ctx, func(*pgxpool.Conn) error {
				t.Fatal("acquire callback must not run")
				return nil
			}), test.want)
			require.ErrorIs(t, database.Ping(ctx), test.want)
		})
	}
}

func TestActiveScopeGuards(t *testing.T) {
	database := &DB{}
	ctx := context.WithValue(context.Background(), scopeKey{}, &transactionScope{})
	_, _, err := database.BeginScope(ctx, pgx.TxOptions{})
	require.ErrorIs(t, err, ErrScopeExists)
	_, err = database.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	require.ErrorIs(t, err, ErrNestedTxOptions)
	_, err = database.Acquire(ctx)
	require.ErrorIs(t, err, ErrScopeAcquire)
	require.ErrorIs(t, database.AcquireFunc(ctx, func(*pgxpool.Conn) error {
		t.Fatal("acquire callback must not run")
		return nil
	}), ErrScopeAcquire)
}
