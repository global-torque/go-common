package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"

	"github.com/global-torque/go-common/configurator"
	"github.com/global-torque/go-common/logger"
)

// NewPool is constructor for pgxpool.Pool
func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	log := logger.NewComponentLogger(ctx, pkgName)
	pgConfig, err := GetConfigPool(log)
	if err != nil {
		return nil, err
	}

	return NewPoolFromConfig(ctx, pgConfig, log)
}

// NewPoolFromConfig is constructor for pgxpool.Pool
func NewPoolFromConfig(ctx context.Context, pgConfig *pgxpool.Config, log logger.Logger) (*pgxpool.Pool, error) {
	if pgConfig == nil {
		return nil, fmt.Errorf("pgx pool config is nil")
	}

	cfg := pgConfig.Copy()
	cfg.AfterConnect = wrapAfterConnect(cfg.AfterConnect)

	return newPool(ctx, cfg, log, retriesFromEnv())
}

func newPool(ctx context.Context, pgConfig *pgxpool.Config, log logger.Logger, retries int) (*pgxpool.Pool, error) {
	if retries < 0 {
		return nil, fmt.Errorf("db max retries must be non-negative")
	}

	pg, err := backoff.RetryWithData(
		func() (*pgxpool.Pool, error) {
			log.Debug().Msg("Connecting to db")
			pool, poolErr := pgxpool.NewWithConfig(ctx, pgConfig)
			if poolErr != nil {
				log.Error().Err(poolErr).Msg("Failed to create connection pool")
				return nil, poolErr
			}

			if pingErr := pool.Ping(ctx); pingErr != nil {
				pool.Close()
				log.Error().Err(pingErr).Msg("Failed to ping db")
				return nil, pingErr
			}

			return pool, nil
		},
		backoff.WithContext(
			backoff.WithMaxRetries(backoff.NewExponentialBackOff(), uint64(retries)),
			ctx,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	return pg, nil
}

func GetConfigPool(log logger.Logger) (*pgxpool.Config, error) {
	cfg, err := configurator.Parse[Config](pkgName)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	err = validateDBConfig(cfg)
	if err != nil {
		return nil, err
	}

	pgConfig, err := pgxpool.ParseConfig(GetPoolConnString(&cfg))
	if err != nil {
		return nil, fmt.Errorf("parse pgx pool config: %w", err)
	}

	configureConnTracing(pgConfig.ConnConfig, cfg.LogLevel, log)
	pgConfig.MaxConns = int32(cfg.MaxConnections)
	pgConfig.MinConns = int32(cfg.MinConnections)
	pgConfig.MaxConnLifetime = fmtDurationSeconds(cfg.MaxConnLifetime)

	return pgConfig, nil
}

func configureConnTracing(pgConfig *pgx.ConnConfig, logLevel string, log logger.Logger) {
	pgxLogLevel, err := tracelog.LogLevelFromString(strings.ToLower(logLevel))
	if err != nil {
		log.Warn().Err(fmt.Errorf("wrong level %q: %w", logLevel, err)).Msg("cannot parse pgx log level")
		pgxLogLevel = tracelog.LogLevelNone
	}

	pgConfig.Tracer = &tracelog.TraceLog{
		Logger:   NewDBLogger(log),
		LogLevel: pgxLogLevel,
	}
}

func GetPoolConnString(cfg *Config) string {
	return GetConnString(cfg)
}

func wrapAfterConnect(next func(context.Context, *pgx.Conn) error) func(context.Context, *pgx.Conn) error {
	return func(ctx context.Context, conn *pgx.Conn) error {
		if next != nil {
			if err := next(ctx, conn); err != nil {
				return err
			}
		}

		return setSessionTimeZone(ctx, conn)
	}
}

func setSessionTimeZone(ctx context.Context, conn interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}) error {
	if _, err := conn.Exec(ctx, "SET TIME ZONE 'UTC'"); err != nil {
		return fmt.Errorf("set db session time zone: %w", err)
	}

	return nil
}

func fmtDurationSeconds(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
