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

type testModel struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func (m testModel) Fields() []string { return DefaultFields(&m) }
func (m testModel) Table() string    { return "test_models" }
func (m *testModel) SetID(id any)    { m.ID = id.(int) }

type stubRepository struct {
	query    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRow func(ctx context.Context, sql string, args ...any) pgx.Row
	exec     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r stubRepository) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return r.query(ctx, sql, args...)
}

func (r stubRepository) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return r.exec(ctx, sql, args...)
}

func (r stubRepository) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return r.queryRow(ctx, sql, args...)
}

type stubRows struct {
	fields  []string
	values  [][]any
	idx     int
	err     error
	scanErr error
}

func (r *stubRows) Close()                        {}
func (r *stubRows) Err() error                    { return r.err }
func (r *stubRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *stubRows) FieldDescriptions() []pgconn.FieldDescription {
	fields := make([]pgconn.FieldDescription, len(r.fields))
	for i, field := range r.fields {
		fields[i] = pgconn.FieldDescription{Name: field}
	}
	return fields
}
func (r *stubRows) Next() bool {
	if r.err != nil || r.idx >= len(r.values) {
		return false
	}
	r.idx++
	return true
}
func (r *stubRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.values[r.idx-1]
	for i := range dest {
		if row[i] == nil {
			continue
		}
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			continue
		}
		value := reflect.ValueOf(row[i])
		if value.Type().AssignableTo(target.Elem().Type()) {
			target.Elem().Set(value)
		} else if value.Type().ConvertibleTo(target.Elem().Type()) {
			target.Elem().Set(value.Convert(target.Elem().Type()))
		}
	}
	return nil
}
func (r *stubRows) Values() ([]any, error) { return r.values[r.idx-1], nil }
func (r *stubRows) RawValues() [][]byte    { return nil }
func (r *stubRows) Conn() *pgx.Conn        { return nil }

type stubRow struct {
	values []any
	err    error
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			continue
		}
		value := reflect.ValueOf(r.values[i])
		if value.Type().AssignableTo(target.Elem().Type()) {
			target.Elem().Set(value)
		} else if value.Type().ConvertibleTo(target.Elem().Type()) {
			target.Elem().Set(value.Convert(target.Elem().Type()))
		} else if target.Elem().Kind() == reflect.Interface {
			target.Elem().Set(value)
		}
	}
	return nil
}

type badSqlizer struct {
	err error
}

func (s badSqlizer) ToSql() (string, []interface{}, error) {
	return "", nil, s.err
}

func TestRetrieveOneBuildsSQLWithSuffix(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	repo := stubRepository{
		query: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			gotSQL = sql
			gotArgs = args
			return &stubRows{
				fields: []string{"id", "name"},
				values: [][]any{{7, "alice"}},
			}, nil
		},
	}

	got, err := RetrieveOne[testModel](context.Background(), repo, sq.Eq{"name": "alice"}, sq.Expr("ORDER BY id LIMIT ?", 1))
	if err != nil {
		t.Fatalf("RetrieveOne returned error: %v", err)
	}

	wantSQL := "SELECT id,name FROM test_models WHERE name = $1 ORDER BY id LIMIT $2"
	if gotSQL != wantSQL {
		t.Fatalf("unexpected SQL:\n got: %s\nwant: %s", gotSQL, wantSQL)
	}
	if !reflect.DeepEqual(gotArgs, []any{"alice", 1}) {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
	if got.ID != 7 || got.Name != "alice" {
		t.Fatalf("unexpected model: %#v", got)
	}
}

func TestRetrieveAllReturnsEmptySlice(t *testing.T) {
	repo := stubRepository{
		query: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &stubRows{fields: []string{"id", "name"}}, nil
		},
	}

	got, err := RetrieveAll[testModel](context.Background(), repo, sq.Eq{"id": 1})
	if err != nil {
		t.Fatalf("RetrieveAll returned error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil slice, got %#v", got)
	}
}

func TestRetrieveOneNotFoundWrapsSentinels(t *testing.T) {
	repo := stubRepository{
		query: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &stubRows{fields: []string{"id", "name"}}, nil
		},
	}

	_, err := RetrieveOne[testModel](context.Background(), repo, sq.Eq{"id": 1})
	if err == nil {
		t.Fatalf("RetrieveOne returned nil error")
	}
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("RetrieveOne error does not wrap ErrRecordNotFound: %v", err)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("RetrieveOne error does not wrap pgx.ErrNoRows: %v", err)
	}
}

func TestCreateBuildsSQLAndSetsID(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	repo := stubRepository{
		queryRow: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return stubRow{values: []any{9}}
		},
	}

	got, err := Create[testModel](context.Background(), repo, map[string]any{"name": "alice"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if gotSQL != "INSERT INTO test_models (name) VALUES ($1) RETURNING id" {
		t.Fatalf("unexpected SQL: %s", gotSQL)
	}
	if !reflect.DeepEqual(gotArgs, []any{"alice"}) {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
	if got.ID != 9 {
		t.Fatalf("unexpected id: %d", got.ID)
	}
}

func TestCreateRejectsEmptyValues(t *testing.T) {
	_, err := Create[testModel](context.Background(), stubRepository{}, nil)
	if !errors.Is(err, ErrEmptyValues) {
		t.Fatalf("expected ErrEmptyValues, got %v", err)
	}
}

func TestUpdateRejectsEmptyPredicate(t *testing.T) {
	_, err := Update[testModel](context.Background(), stubRepository{}, nil, map[string]any{"name": "alice"})
	if !errors.Is(err, ErrEmptyPredicate) {
		t.Fatalf("expected empty predicate error, got %v", err)
	}

	_, err = Update[testModel](context.Background(), stubRepository{}, nil, map[string]any{"name": "alice"}, sq.Expr("1=1"))
	if !errors.Is(err, ErrEmptyPredicate) {
		t.Fatalf("expected tautology predicate error, got %v", err)
	}
}

func TestUpdateWithExpressionPredicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	repo := stubRepository{
		exec: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL = sql
			gotArgs = args
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}

	updated, err := Update[testModel](
		context.Background(),
		repo,
		nil,
		map[string]any{"name": "alice"},
		sq.Expr("id = ?", 7),
	)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if !updated {
		t.Fatalf("expected updated")
	}
	if gotSQL != "UPDATE test_models SET name = $1 WHERE id = $2" {
		t.Fatalf("unexpected SQL: %s", gotSQL)
	}
	if !reflect.DeepEqual(gotArgs, []any{"alice", 7}) {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
}

func TestUpdateAffectedRows(t *testing.T) {
	tests := []struct {
		name       string
		commandTag string
		want       bool
	}{
		{name: "zero", commandTag: "UPDATE 0"},
		{name: "one", commandTag: "UPDATE 1", want: true},
		{name: "multiple", commandTag: "UPDATE 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := stubRepository{
				exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag(tt.commandTag), nil
				},
			}

			updated, err := Update[testModel](
				context.Background(),
				repo,
				map[string]any{"id": 7},
				map[string]any{"name": "alice"},
			)
			if err != nil {
				t.Fatalf("Update returned error: %v", err)
			}
			if updated != tt.want {
				t.Fatalf("Update returned %v, want %v", updated, tt.want)
			}
		})
	}
}

func TestDeleteRejectsEmptyPredicate(t *testing.T) {
	_, err := Delete[testModel](context.Background(), stubRepository{}, nil)
	if !errors.Is(err, ErrEmptyPredicate) {
		t.Fatalf("expected empty predicate error, got %v", err)
	}
}

func TestDeleteAffectedRows(t *testing.T) {
	tests := []struct {
		name       string
		commandTag string
		want       bool
	}{
		{name: "zero", commandTag: "DELETE 0"},
		{name: "one", commandTag: "DELETE 1", want: true},
		{name: "multiple", commandTag: "DELETE 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := stubRepository{
				exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag(tt.commandTag), nil
				},
			}

			deleted, err := Delete[testModel](context.Background(), repo, map[string]any{"id": 7})
			if err != nil {
				t.Fatalf("Delete returned error: %v", err)
			}
			if deleted != tt.want {
				t.Fatalf("Delete returned %v, want %v", deleted, tt.want)
			}
		})
	}
}

func TestExistsReturnsFalseOnNoRows(t *testing.T) {
	repo := stubRepository{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return stubRow{err: pgx.ErrNoRows}
		},
	}

	exists, err := Exists[testModel](context.Background(), repo, map[string]any{"id": 1})
	if err != nil {
		t.Fatalf("Exists returned error: %v", err)
	}
	if exists {
		t.Fatalf("expected false")
	}
}

func TestBadSqlizerErrorIsWrapped(t *testing.T) {
	prepareErr := errors.New("prepare failed")
	_, err := RetrieveOne[testModel](context.Background(), stubRepository{}, badSqlizer{err: prepareErr})
	if !errors.Is(err, prepareErr) {
		t.Fatalf("expected wrapped prepare error, got %v", err)
	}
}
