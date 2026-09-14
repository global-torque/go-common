//nolint:ireturn // Adapter methods preserve pgx's public interface return types.
package db

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrScopeExists     = errors.New("db: transaction scope already exists")
	ErrScopePool       = errors.New("db: transaction scope belongs to another pool")
	ErrScopeAcquire    = errors.New("db: cannot acquire a connection inside a transaction scope")
	ErrNestedTxOptions = errors.New("db: nested transactions cannot specify transaction options")
)

type scopeKey struct{}

type transactionScope struct {
	pgx.Tx

	pool   *pgxpool.Pool
	closed atomic.Bool
}

func (s *transactionScope) Commit(ctx context.Context) error {
	if s.closed.Swap(true) {
		return pgx.ErrTxClosed
	}

	return s.Tx.Commit(ctx)
}

func (s *transactionScope) Rollback(ctx context.Context) error {
	if s.closed.Swap(true) {
		return pgx.ErrTxClosed
	}

	return s.Tx.Rollback(ctx)
}

// BeginScope starts a root transaction and binds it to the returned context.
// Calls through adapters sharing this pool use the transaction until it closes.
// Work in a scope must be sequential, with rows and batches closed before completion.
func (db *DB) BeginScope(ctx context.Context, options pgx.TxOptions) (context.Context, pgx.Tx, error) {
	scope, err := db.scope(ctx)
	if err != nil {
		return nil, nil, err
	}

	if scope != nil {
		return nil, nil, ErrScopeExists
	}

	tx, err := db.Pool.BeginTx(ctx, options)
	if err != nil {
		return nil, nil, err
	}

	scope = &transactionScope{Tx: tx, pool: db.Pool}

	return context.WithValue(ctx, scopeKey{}, scope), scope, nil
}

func (db *DB) scope(ctx context.Context) (*transactionScope, error) {
	scope, ok := ctx.Value(scopeKey{}).(*transactionScope)
	if !ok {
		return nil, nil //nolint:nilnil // An absent scope selects ordinary pool behavior.
	}

	if scope.pool != db.Pool {
		return nil, ErrScopePool
	}

	if scope.closed.Load() {
		return nil, pgx.ErrTxClosed
	}

	return scope, nil
}

// Exec executes SQL using the contextual transaction, or the pool without a scope.
func (db *DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	scope, err := db.scope(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}

	if scope != nil {
		return scope.Exec(ctx, sql, args...)
	}

	return db.Pool.Exec(ctx, sql, args...)
}

// Query queries using the contextual transaction, or the pool without a scope.
func (db *DB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	scope, err := db.scope(ctx)
	if err != nil {
		return nil, err
	}

	if scope != nil {
		return scope.Query(ctx, sql, args...)
	}

	return db.Pool.Query(ctx, sql, args...)
}

// QueryRow queries one row using the contextual transaction, or the pool without a scope.
func (db *DB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	scope, err := db.scope(ctx)
	if err != nil {
		return failedRow{err: err}
	}

	if scope != nil {
		return scope.QueryRow(ctx, sql, args...)
	}

	return db.Pool.QueryRow(ctx, sql, args...)
}

// SendBatch sends a batch using the contextual transaction, or the pool without a scope.
func (db *DB) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	scope, err := db.scope(ctx)
	if err != nil {
		return failedBatch{err: err}
	}

	if scope != nil {
		return scope.SendBatch(ctx, batch)
	}

	return db.Pool.SendBatch(ctx, batch)
}

// CopyFrom copies rows using the contextual transaction, or the pool without a scope.
func (db *DB) CopyFrom(
	ctx context.Context, table pgx.Identifier, columns []string, rows pgx.CopyFromSource,
) (int64, error) {
	scope, err := db.scope(ctx)
	if err != nil {
		return 0, err
	}

	if scope != nil {
		return scope.CopyFrom(ctx, table, columns, rows)
	}

	return db.Pool.CopyFrom(ctx, table, columns, rows)
}

// Begin starts a transaction, or a savepoint inside a transaction scope.
func (db *DB) Begin(ctx context.Context) (pgx.Tx, error) {
	return db.BeginTx(ctx, pgx.TxOptions{})
}

// BeginTx starts a transaction, or a savepoint when options are zero inside a scope.
func (db *DB) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	scope, err := db.scope(ctx)
	if err != nil {
		return nil, err
	}

	if scope != nil {
		if options != (pgx.TxOptions{}) {
			return nil, ErrNestedTxOptions
		}

		return scope.Begin(ctx)
	}

	return db.Pool.BeginTx(ctx, options)
}

// Acquire acquires a connection only when there is no contextual transaction.
func (db *DB) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	scope, err := db.scope(ctx)
	if err != nil {
		return nil, err
	}

	if scope != nil {
		return nil, ErrScopeAcquire
	}

	return db.Pool.Acquire(ctx)
}

// AcquireFunc acquires a connection only when there is no contextual transaction.
func (db *DB) AcquireFunc(ctx context.Context, fn func(*pgxpool.Conn) error) error {
	scope, err := db.scope(ctx)
	if err != nil {
		return err
	}

	if scope != nil {
		return ErrScopeAcquire
	}

	return db.Pool.AcquireFunc(ctx, fn)
}

// Ping checks the connection already held by a scope, or pings the pool without one.
func (db *DB) Ping(ctx context.Context) error {
	scope, err := db.scope(ctx)
	if err != nil {
		return err
	}

	if scope != nil {
		_, err = scope.Exec(ctx, "select 1")
		return err
	}

	return db.Pool.Ping(ctx)
}

type failedRow struct{ err error }

func (r failedRow) Scan(...any) error { return r.err }

type failedBatch struct{ err error }

func (b failedBatch) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, b.err }
func (b failedBatch) Query() (pgx.Rows, error)         { return nil, b.err }
func (b failedBatch) QueryRow() pgx.Row                { return failedRow(b) }
func (b failedBatch) Close() error                     { return b.err }
