package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/global-torque/go-common/logger"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/tracelog"
)

// Logger is a struct that represent logger for DB
type Logger struct {
	log logger.Logger
}

// NewDBLogger is a constructor for Logger
func NewDBLogger(log logger.Logger) *Logger {
	return &Logger{log: log}
}

// Log prints a message
func (l *Logger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
	var err error
	errMsg, ok := data["err"]
	if ok {
		switch val := errMsg.(type) {
		case *pgconn.PgError:
			msg = val.Message
			err = val
		default:
			msg = fmt.Sprintf("%s", val)
			err, _ = errMsg.(error)
		}
	}

	switch level {
	case tracelog.LogLevelNone:
	case tracelog.LogLevelTrace:
		l.log.Trace().Ctx(ctx).Interface("data", data).Msg(msg)
	case tracelog.LogLevelDebug:
		l.log.Debug().Ctx(ctx).Interface("data", data).Msg(msg)
	case tracelog.LogLevelInfo:
		if len(msg) > 4 && msg[0:5] == "Query" {
			l.log.Info().Ctx(ctx).Interface("data", data).Msg(msg)
		}
	case tracelog.LogLevelWarn:
		l.log.Warn().Ctx(ctx).Interface("data", data).Msg(msg)
	case tracelog.LogLevelError:
		l.log.Error().Stack().Ctx(ctx).Err(err).Interface("data", data).Msg(msg)
	}
}

func CleanSQL(query string) string {
	q := strings.ReplaceAll(query, "\t", " ")
	q = strings.ReplaceAll(q, "\n", " ")
	q = strings.ReplaceAll(q, "  ", " ")
	q = strings.ReplaceAll(q, "  ", " ")
	return q
}

// LogQuery custom method to log SQL, for future use in Log
func (db *DB) LogQuery(ctx context.Context, query string, args interface{}) {
	// ToDo
	// Replace $1,$2 with values
	q := CleanSQL(query)
	db.Log.Trace().Ctx(ctx).Msgf("query: %s, %v", q, args)
}

func (db *DB) Lg() logger.Logger {
	return db.Log
}
