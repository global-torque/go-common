package orm

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrSQLBuild indicates a query could not be converted into SQL.
	ErrSQLBuild = errors.New("error during sql prepare")
	// ErrEmptyPredicate is returned when a destructive query has no real predicate.
	ErrEmptyPredicate = errors.New("empty predicate is not allowed")
	// ErrRecordNotFound is returned when a query expects one record but none exist.
	ErrRecordNotFound = fmt.Errorf("record not found: %w", pgx.ErrNoRows)
	// ErrEmptyValues indicates that an insert/update payload has no values.
	ErrEmptyValues = errors.New("empty values")
	// ErrNoChanges indicates that a Save update has no changed values.
	ErrNoChanges = errors.New("no changes")
	// ErrEmptyID indicates that a model save/update was requested without an ID.
	ErrEmptyID = errors.New("model id is not set")
	// ErrNoRowsAffected indicates that an expected update/delete did not affect any row.
	ErrNoRowsAffected = errors.New("no rows affected")
	// ErrIntegrity indicates an impossible persistence result such as multiple primary-key updates.
	ErrIntegrity = errors.New("orm integrity error")
	// ErrInvalidProjection indicates that a typed projection does not have a
	// one-to-one explicit db-tag mapping to columns on its source model.
	ErrInvalidProjection = errors.New("invalid projection")
	// ErrTxBegin indicates that a transaction could not be started.
	ErrTxBegin = errors.New("begin transaction")
	// ErrTxCallback indicates that a transaction callback returned an error.
	ErrTxCallback = errors.New("transaction callback")
	// ErrTxRollback indicates that a transaction rollback failed.
	ErrTxRollback = errors.New("rollback transaction")
	// ErrTxCommit indicates that a transaction commit failed.
	ErrTxCommit = errors.New("commit transaction")
	// ErrTxPanic indicates that a transaction callback panicked.
	ErrTxPanic = errors.New("transaction callback panic")
)

const (
	msgRetrieveOne = "cannot retrieve one record"
	msgRetrieveAll = "cannot retrieve records"
	msgCreate      = "cannot create record"
	msgUpdate      = "cannot update record"
	msgDelete      = "cannot delete record"
)
