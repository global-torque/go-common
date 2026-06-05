package orm

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeBeginner struct {
	tx  *fakeTx
	err error
}

func (b fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	return b.tx, b.err
}

type fakeTx struct {
	commitErr   error
	rollbackErr error
	committed   bool
	rolledBack  bool
	rollbackCtx context.Context
}

func (tx *fakeTx) Begin(context.Context) (pgx.Tx, error) { return tx, nil }
func (tx *fakeTx) Commit(context.Context) error {
	tx.committed = true
	return tx.commitErr
}
func (tx *fakeTx) Rollback(ctx context.Context) error {
	tx.rolledBack = true
	tx.rollbackCtx = ctx
	return tx.rollbackErr
}
func (tx *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (tx *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (tx *fakeTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (tx *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (tx *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (tx *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (tx *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }
func (tx *fakeTx) Conn() *pgx.Conn                                         { return nil }

func TestWithTxCommit(t *testing.T) {
	tx := &fakeTx{}
	err := WithTx(context.Background(), fakeBeginner{tx: tx}, func(context.Context, Repository) error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithTx returned error: %v", err)
	}
	if !tx.committed || tx.rolledBack {
		t.Fatalf("unexpected tx state: %#v", tx)
	}
}

func TestWithTxRollbackOnCallbackError(t *testing.T) {
	tx := &fakeTx{}
	callbackErr := errors.New("callback failed")
	err := WithTx(context.Background(), fakeBeginner{tx: tx}, func(context.Context, Repository) error {
		return callbackErr
	})
	if !errors.Is(err, callbackErr) || !errors.Is(err, ErrTxCallback) {
		t.Fatalf("expected callback error, got %v", err)
	}
	if !tx.rolledBack || tx.committed {
		t.Fatalf("unexpected tx state: %#v", tx)
	}
}

func TestWithTxRollbackUsesUncancelledContext(t *testing.T) {
	tx := &fakeTx{}
	ctx, cancel := context.WithCancel(context.Background())

	err := WithTx(ctx, fakeBeginner{tx: tx}, func(context.Context, Repository) error {
		cancel()
		return context.Canceled
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected callback cancellation error, got %v", err)
	}
	if tx.rollbackCtx == nil {
		t.Fatalf("Rollback was not called")
	}
	if rollbackErr := tx.rollbackCtx.Err(); rollbackErr != nil {
		t.Fatalf("Rollback received cancelled context: %v", rollbackErr)
	}
}

func TestWithTxRollbackOnPanic(t *testing.T) {
	tx := &fakeTx{}
	err := WithTx(context.Background(), fakeBeginner{tx: tx}, func(context.Context, Repository) error {
		panic("boom")
	})
	if !errors.Is(err, ErrTxPanic) {
		t.Fatalf("expected panic error, got %v", err)
	}
	if !tx.rolledBack || tx.committed {
		t.Fatalf("unexpected tx state: %#v", tx)
	}
}

func TestWithTxCommitError(t *testing.T) {
	commitErr := errors.New("commit failed")
	tx := &fakeTx{commitErr: commitErr}
	err := WithTx(context.Background(), fakeBeginner{tx: tx}, func(context.Context, Repository) error {
		return nil
	})
	if !errors.Is(err, commitErr) || !errors.Is(err, ErrTxCommit) {
		t.Fatalf("expected commit error, got %v", err)
	}
}
