package orm

import (
	"context"
	"fmt"
)

// WithTx runs fn inside a transaction and handles rollback or commit.
func WithTx(ctx context.Context, beginner Beginner, fn func(ctx context.Context, tx Repository) error) (err error) {
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTxBegin, err)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}

		rollbackErr := tx.Rollback(context.WithoutCancel(ctx))
		if rollbackErr != nil {
			err = fmt.Errorf("%w: %v: %w: %w", ErrTxPanic, recovered, ErrTxRollback, rollbackErr)
			return
		}

		err = fmt.Errorf("%w: %v", ErrTxPanic, recovered)
	}()

	err = fn(ctx, tx)
	if err != nil {
		rollbackErr := tx.Rollback(context.WithoutCancel(ctx))
		if rollbackErr != nil {
			return fmt.Errorf("%w: %w: %w: %w", ErrTxCallback, err, ErrTxRollback, rollbackErr)
		}

		return fmt.Errorf("%w: %w", ErrTxCallback, err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTxCommit, err)
	}

	return nil
}
