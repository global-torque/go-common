package orm

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Repository is the minimal query surface required by the ORM helpers.
type Repository interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Beginner starts a pgx transaction.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}
