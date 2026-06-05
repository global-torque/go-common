//nolint:wsl_v5 // This package follows compact go-common query-builder style.
package orm

import (
	"context"
	"fmt"
	"reflect"

	sq "github.com/Masterminds/squirrel"
)

// SaveModel is the explicit persistence contract used by Save.
type SaveModel interface {
	Table() string
	GetID() any
	SetID(id any)
	InsertValues() map[string]any
	Changes() map[string]any
}

// PrimaryKeyer overrides the default primary key column.
type PrimaryKeyer interface {
	PrimaryKey() string
}

// ChangeResetter clears tracked changes after a successful save.
type ChangeResetter interface {
	ResetChanges()
}

// SaveResult describes the persistence action performed by Save.
type SaveResult struct {
	Action       SaveAction
	Table        string
	PrimaryKey   string
	ID           any
	RowsAffected int64
}

// SaveAction describes whether Save inserted or updated a row.
type SaveAction string

const (
	// SaveInserted means Save inserted a new row.
	SaveInserted SaveAction = "inserted"
	// SaveUpdated means Save updated an existing row.
	SaveUpdated SaveAction = "updated"
)

// Save inserts or updates one model using explicit model-provided values.
func Save(ctx context.Context, repo Repository, model SaveModel) (SaveResult, error) {
	result := SaveResult{
		Table:      model.Table(),
		PrimaryKey: primaryKey(model),
		ID:         model.GetID(),
	}

	if isZero(model.GetID()) {
		return saveInsert(ctx, repo, model, result)
	}
	return saveUpdate(ctx, repo, model, result)
}

func saveInsert(ctx context.Context, repo Repository, model SaveModel, result SaveResult) (SaveResult, error) {
	values := model.InsertValues()
	if len(values) == 0 {
		return result, fmt.Errorf("%w: insert %s", ErrEmptyValues, result.Table)
	}

	sql, args, err := sq.Insert(result.Table).
		SetMap(values).
		Suffix("RETURNING " + result.PrimaryKey).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return result, fmt.Errorf("%w: insert %s: %w", ErrSQLBuild, result.Table, err)
	}

	var id int
	scanErr := repo.QueryRow(ctx, sql, args...).Scan(&id)
	if scanErr != nil {
		return result, fmt.Errorf("%s: insert %s: %w", msgCreate, result.Table, scanErr)
	}

	model.SetID(id)
	result.Action = SaveInserted
	result.ID = id
	result.RowsAffected = 1

	resetChanges(model)

	return result, nil
}

func saveUpdate(ctx context.Context, repo Repository, model SaveModel, result SaveResult) (SaveResult, error) {
	changes := model.Changes()
	if len(changes) == 0 {
		return result, fmt.Errorf("%w: update %s", ErrNoChanges, result.Table)
	}

	sql, args, err := sq.Update(result.Table).
		SetMap(changes).
		Where(sq.Eq{result.PrimaryKey: model.GetID()}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return result, fmt.Errorf("%w: update %s: %w", ErrSQLBuild, result.Table, err)
	}

	tag, err := repo.Exec(ctx, sql, args...)
	if err != nil {
		return result, fmt.Errorf("%s: update %s: %w", msgUpdate, result.Table, err)
	}

	result.Action = SaveUpdated
	result.ID = model.GetID()
	result.RowsAffected = tag.RowsAffected()

	switch {
	case result.RowsAffected == 0:
		return result, fmt.Errorf("%w: %s %v", ErrRecordNotFound, result.PrimaryKey, result.ID)
	case result.RowsAffected > 1:
		return result, fmt.Errorf("%w: update %s affected %d rows", ErrIntegrity, result.Table, result.RowsAffected)
	}

	resetChanges(model)

	return result, nil
}

func primaryKey(model SaveModel) string {
	if keyed, ok := model.(PrimaryKeyer); ok {
		if key := keyed.PrimaryKey(); key != "" {
			return key
		}
	}
	return "id"
}

func resetChanges(model SaveModel) {
	if resetter, ok := model.(ChangeResetter); ok {
		resetter.ResetChanges()
	}
}

func isZero(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	return !rv.IsValid() || rv.IsZero()
}
