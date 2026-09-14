package db_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/global-torque/go-common/db/v2"
	"github.com/global-torque/go-common/orm/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func scopeDatabase(t *testing.T, maxConnections int32) (*db.DB, context.Context) {
	t.Helper()
	url := os.Getenv("ATOMIC_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ATOMIC_TEST_DATABASE_URL is required for PostgreSQL scope integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	config, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)
	config.MaxConns = maxConnections
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))
	return &db.DB{Pool: pool}, ctx
}

func TestScopePostgresOperations(t *testing.T) {
	for _, commit := range []bool{true, false} {
		t.Run(fmt.Sprintf("commit=%t", commit), func(t *testing.T) {
			database, ctx := scopeDatabase(t, 1)
			_, err := database.Exec(ctx, "create temp table scope_writes (id integer)")
			require.NoError(t, err)
			scoped, tx, err := database.BeginScope(ctx, pgx.TxOptions{})
			require.NoError(t, err)
			defer func() { _ = tx.Rollback(context.Background()) }()
			require.NoError(t, database.Ping(scoped))
			_, err = database.Exec(scoped, "insert into scope_writes values (1)")
			require.NoError(t, err)
			batch := &pgx.Batch{}
			batch.Queue("insert into scope_writes values (2)")
			require.NoError(t, database.SendBatch(scoped, batch).Close())
			count, err := database.CopyFrom(scoped, pgx.Identifier{"scope_writes"}, []string{"id"},
				pgx.CopyFromRows([][]any{{3}}))
			require.NoError(t, err)
			require.EqualValues(t, 1, count)
			rows, err := database.Query(scoped, "select id from scope_writes order by id")
			require.NoError(t, err)
			values, err := pgx.CollectRows(rows, pgx.RowTo[int])
			require.NoError(t, err)
			require.Equal(t, []int{1, 2, 3}, values)
			alias := &db.DB{Pool: database.Pool}
			var actual int
			require.NoError(t, alias.QueryRow(scoped, "select count(*) from scope_writes").Scan(&actual))
			require.Equal(t, 3, actual)
			if commit {
				require.NoError(t, tx.Commit(ctx))
			} else {
				require.NoError(t, tx.Rollback(ctx))
			}
			require.ErrorIs(t, database.QueryRow(scoped, "select 1").Scan(&actual), pgx.ErrTxClosed)
			require.NoError(t, database.QueryRow(ctx, "select count(*) from scope_writes").Scan(&actual))
			if commit {
				require.Equal(t, 3, actual)
			} else {
				require.Zero(t, actual)
			}
		})
	}
}

func TestScopePostgresNestedHelpers(t *testing.T) {
	database, ctx := scopeDatabase(t, 1)
	_, err := database.Exec(ctx, "create temp table nested_writes (id integer)")
	require.NoError(t, err)
	scoped, root, err := database.BeginScope(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = root.Rollback(context.Background()) }()
	_, err = database.Exec(scoped, "insert into nested_writes values (1)")
	require.NoError(t, err)
	first, err := database.Begin(scoped)
	require.NoError(t, err)
	_, err = first.Exec(scoped, "insert into nested_writes values (2)")
	require.NoError(t, err)
	second, err := first.Begin(scoped)
	require.NoError(t, err)
	_, err = second.Exec(scoped, "insert into nested_writes values (3)")
	require.NoError(t, err)
	third, err := second.Begin(scoped)
	require.NoError(t, err)
	_, err = third.Exec(scoped, "insert into nested_writes values (4)")
	require.NoError(t, err)
	require.NoError(t, third.Rollback(scoped))
	require.NoError(t, second.Commit(scoped))
	require.NoError(t, first.Commit(scoped))
	require.NoError(t, orm.WithTx(scoped, database, func(ctx context.Context, tx orm.Repository) error {
		_, err := tx.Exec(ctx, "insert into nested_writes values (5)")
		return err
	}))
	innerFailure := errors.New("inner operation failed")
	err = orm.WithTx(scoped, database, func(ctx context.Context, tx orm.Repository) error {
		if _, err := tx.Exec(ctx, "insert into nested_writes values (6)"); err != nil {
			return err
		}
		return innerFailure
	})
	require.ErrorIs(t, err, innerFailure)
	var actual []int
	require.NoError(t, database.QueryRow(scoped, "select array_agg(id order by id) from nested_writes").Scan(&actual))
	require.Equal(t, []int{1, 2, 3, 5}, actual)
	require.NoError(t, root.Rollback(ctx))
	var count int
	require.NoError(t, database.QueryRow(ctx, "select count(*) from nested_writes").Scan(&count))
	require.Zero(t, count, "releasing savepoints must not commit the root transaction")
}

func TestScopePostgresConcurrentIsolation(t *testing.T) {
	database, ctx := scopeDatabase(t, 2)
	ready := make(chan struct{}, 2)
	proceed := make(chan struct{})
	results := make(chan error, 2)
	for id := range 2 {
		go func() {
			results <- func() error {
				scoped, tx, err := database.BeginScope(ctx, pgx.TxOptions{})
				if err != nil {
					return err
				}
				defer func() { _ = tx.Rollback(context.Background()) }()
				if _, err := database.Exec(scoped, "create temp table isolated_scope (id integer) on commit drop"); err != nil {
					return err
				}
				if _, err := database.Exec(scoped, "insert into isolated_scope values ($1)", id); err != nil {
					return err
				}
				ready <- struct{}{}
				select {
				case <-proceed:
				case <-ctx.Done():
					return ctx.Err()
				}
				var actual int
				if err := database.QueryRow(scoped, "select id from isolated_scope").Scan(&actual); err != nil {
					return err
				}
				if actual != id {
					return fmt.Errorf("scope %d saw %d", id, actual)
				}
				return tx.Commit(ctx)
			}()
		}()
	}
	for range 2 {
		select {
		case <-ready:
		case err := <-results:
			t.Fatalf("transaction failed before barrier: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	close(proceed)
	for range 2 {
		require.NoError(t, <-results)
	}
}
