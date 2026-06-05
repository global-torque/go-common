package orm

import (
	"context"
	"errors"
	"reflect"
	"testing"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type saveModel struct {
	id          any
	table       string
	primaryKey  string
	inserts     map[string]any
	changes     ChangeSet
	resetCalled bool
}

func (m *saveModel) Table() string {
	if m.table != "" {
		return m.table
	}
	return "save_models"
}
func (m *saveModel) GetID() any                    { return m.id }
func (m *saveModel) SetID(id any)                  { m.id = id }
func (m *saveModel) InsertValues() map[string]any  { return m.inserts }
func (m *saveModel) Changes() map[string]any       { return m.changes.Changes() }
func (m *saveModel) ResetChanges()                 { m.resetCalled = true; m.changes.ResetChanges() }
func (m *saveModel) PrimaryKey() string            { return m.primaryKey }
func (m *saveModel) setChange(key string, val any) { m.changes.Set(key, val) }

func TestSaveInsert(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	repo := stubRepository{
		queryRow: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return stubRow{values: []any{11}}
		},
	}
	model := &saveModel{inserts: map[string]any{"name": "alice"}}

	result, err := Save(context.Background(), repo, model)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if gotSQL != "INSERT INTO save_models (name) VALUES ($1) RETURNING id" {
		t.Fatalf("unexpected SQL: %s", gotSQL)
	}
	if !reflect.DeepEqual(gotArgs, []any{"alice"}) {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
	if model.id != 11 {
		t.Fatalf("unexpected id: %#v", model.id)
	}
	if !model.resetCalled {
		t.Fatalf("expected reset")
	}
	if result.Action != SaveInserted || result.RowsAffected != 1 || result.PrimaryKey != "id" || result.ID != 11 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSaveUpdateWithCustomPrimaryKeyAndExpression(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	repo := stubRepository{
		exec: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL = sql
			gotArgs = args
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	model := &saveModel{id: "abc", primaryKey: "external_id"}
	model.setChange("balance", sq.Expr("balance + ?", 10))

	result, err := Save(context.Background(), repo, model)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if gotSQL != "UPDATE save_models SET balance = balance + $1 WHERE external_id = $2" {
		t.Fatalf("unexpected SQL: %s", gotSQL)
	}
	if !reflect.DeepEqual(gotArgs, []any{10, "abc"}) {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
	if !model.resetCalled {
		t.Fatalf("expected reset")
	}
	if result.Action != SaveUpdated || result.RowsAffected != 1 || result.PrimaryKey != "external_id" || result.ID != "abc" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSaveErrors(t *testing.T) {
	tests := []struct {
		name  string
		model *saveModel
		repo  Repository
		want  error
	}{
		{
			name:  "empty insert",
			model: &saveModel{},
			repo:  stubRepository{},
			want:  ErrEmptyValues,
		},
		{
			name:  "empty changes",
			model: &saveModel{id: 1},
			repo:  stubRepository{},
			want:  ErrNoChanges,
		},
		{
			name:  "zero rows",
			model: func() *saveModel { m := &saveModel{id: 1}; m.setChange("name", "alice"); return m }(),
			repo: stubRepository{
				exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag("UPDATE 0"), nil
				},
			},
			want: ErrRecordNotFound,
		},
		{
			name:  "multiple rows",
			model: func() *saveModel { m := &saveModel{id: 1}; m.setChange("name", "alice"); return m }(),
			repo: stubRepository{
				exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag("UPDATE 2"), nil
				},
			},
			want: ErrIntegrity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Save(context.Background(), tt.repo, tt.model)
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}
