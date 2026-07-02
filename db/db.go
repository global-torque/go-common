//nolint:gochecknoglobals
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/global-torque/go-common/logger/v2"
)

var (
	pkgName = "db"
	// maxRetries is the fallback ceiling on initial-connection retries.
	// Per-call retries also honor cfg.MaxRetries (env DB_MAX_RETRIES).
	maxRetries = 5
)

// DB is a layer to simplify interact with DB
type DB struct {
	*pgxpool.Pool
	Log logger.Logger
}

type Repository interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	// LogPubSubMsg(ctx context.Context, topic string, msg *pclient.Message) error
	Lg() logger.Logger
}

// New returns new DB instance.
func New(ctx context.Context) (*DB, error) {
	log := logger.NewComponentLogger(ctx, pkgName)
	pool, err := NewPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	return NewDB(pool, log), nil
}

// MustNew is New with fatal-on-error semantics for app main packages.
func MustNew(ctx context.Context) *DB {
	db, err := New(ctx)
	if err != nil {
		log := logger.NewComponentLogger(ctx, pkgName)
		log.Fatal().Err(err).Msg("failed to connect to db")
	}

	return db
}

// NewDB returns new DB instance.
func NewDB(pool *pgxpool.Pool, log logger.Logger) *DB {
	d := &DB{
		Pool: pool,
		Log:  log,
	}

	return d
}

// Subscribe is
func (db *DB) Subscribe(ctx context.Context, topicName string) (<-chan []byte, error) {
	conn, err := db.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	topic := pgx.Identifier{topicName}.Sanitize()
	if _, err := conn.Exec(ctx, "listen "+topic); err != nil {
		conn.Release()
		return nil, err
	}

	out := make(chan []byte)

	go func() {
		defer func() {
			if _, err := conn.Exec(context.Background(), "unlisten "+topic); err != nil {
				db.Log.Error().Err(err).Str("topic", topicName).Msg("can't unlisten notification topic")
			}
			conn.Release()
		}()
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := conn.Conn().WaitForNotification(ctx)
				if err != nil {
					db.Log.Error().Err(err).Msg("Can't receive notification, continuing")

					if conn.Conn().IsClosed() {
						db.Log.Error().Err(err).Msg("Lost connection")
						return
					}

					continue
				}

				payload := []byte(n.Payload)
				select {
				case out <- payload:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}
