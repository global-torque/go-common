//nolint:wsl_v5 // This package follows compact go-common query-builder style.
package orm

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type queryModel[T any] interface {
	*T
	SetID(id any)
	Fields() []string
	Table() string
}

// RetrieveOneAs fetches one source row into a strict, explicitly tagged
// projection. Projection columns must exist on the canonical source model.
func RetrieveOneAs[Source any, Projection any, PSource queryModel[Source]](
	ctx context.Context,
	repo Repository,
	where sq.Sqlizer,
	suffixes ...sq.Sqlizer,
) (*Projection, error) {
	obj := PSource(new(Source))
	result := new(Projection)
	fields, err := projectionFields[Source, Projection]()
	if err != nil {
		return result, err
	}

	builder := sq.Select(strings.Join(fields, ",")).From(obj.Table())
	if where != nil {
		builder = builder.Where(where)
	}
	for _, suffix := range suffixes {
		if suffix != nil {
			builder = builder.SuffixExpr(suffix)
		}
	}

	sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return result, fmt.Errorf("%w: %s, %+v: %w", ErrSQLBuild, where, suffixes, err)
	}

	rows, err := repo.Query(ctx, sql, args...)
	if err != nil {
		return result, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveOne, sql, args, err)
	}

	value, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Projection])
	if err != nil {
		if errorsIsNoRows(err) {
			return &value, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveOne, sql, args, ErrRecordNotFound)
		}
		return &value, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveOne, sql, args, err)
	}

	return &value, nil
}

// RetrieveAllAs fetches source rows into strict, explicitly tagged
// projections. It returns a non-nil empty slice when no rows match.
func RetrieveAllAs[Source any, Projection any, PSource queryModel[Source]](
	ctx context.Context,
	repo Repository,
	where sq.Sqlizer,
	suffixes ...sq.Sqlizer,
) ([]*Projection, error) {
	obj := PSource(new(Source))
	fields, err := projectionFields[Source, Projection]()
	if err != nil {
		return nil, err
	}

	builder := sq.Select(strings.Join(fields, ",")).From(obj.Table())
	if where != nil {
		builder = builder.Where(where)
	}
	for _, suffix := range suffixes {
		if suffix != nil {
			builder = builder.SuffixExpr(suffix)
		}
	}

	sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("%w: %s, %+v: %w", ErrSQLBuild, where, suffixes, err)
	}

	rows, err := repo.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveAll, sql, args, err)
	}

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[Projection])
	if err != nil {
		return results, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveAll, sql, args, err)
	}
	if results == nil {
		return []*Projection{}, nil
	}
	return results, nil
}

// RetrieveOne fetches one model by a squirrel predicate and optional suffixes.
func RetrieveOne[T any, PT queryModel[T]](
	ctx context.Context,
	repo Repository,
	where sq.Sqlizer,
	suffixes ...sq.Sqlizer,
) (*T, error) {
	obj := PT(new(T))

	builder := sq.Select(strings.Join(obj.Fields(), ",")).From(obj.Table())
	if where != nil {
		builder = builder.Where(where)
	}
	for _, suffix := range suffixes {
		if suffix != nil {
			builder = builder.SuffixExpr(suffix)
		}
	}

	sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return obj, fmt.Errorf("%w: %s, %+v: %w", ErrSQLBuild, where, suffixes, err)
	}

	rows, err := repo.Query(ctx, sql, args...)
	if err != nil {
		return obj, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveOne, sql, args, err)
	}

	result, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		if errorsIsNoRows(err) {
			return &result, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveOne, sql, args, ErrRecordNotFound)
		}
		return &result, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveOne, sql, args, err)
	}

	return &result, nil
}

// RetrieveAll fetches all models matching a squirrel predicate and optional suffixes.
func RetrieveAll[T any, PT queryModel[T]](
	ctx context.Context,
	repo Repository,
	where sq.Sqlizer,
	suffixes ...sq.Sqlizer,
) ([]*T, error) {
	obj := PT(new(T))

	builder := sq.Select(strings.Join(obj.Fields(), ",")).From(obj.Table())
	if where != nil {
		builder = builder.Where(where)
	}
	for _, suffix := range suffixes {
		if suffix != nil {
			builder = builder.SuffixExpr(suffix)
		}
	}

	sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("%w: %s, %+v: %w", ErrSQLBuild, where, suffixes, err)
	}

	rows, err := repo.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveAll, sql, args, err)
	}

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[T])
	if err != nil {
		return results, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveAll, sql, args, err)
	}
	if results == nil {
		return []*T{}, nil
	}
	return results, nil
}

// Create inserts data into a model table and scans the generated integer ID.
func Create[T any, PT queryModel[T]](ctx context.Context, repo Repository, data map[string]any) (*T, error) {
	obj := PT(new(T))
	if len(data) == 0 {
		return obj, fmt.Errorf("%s: %w", msgCreate, ErrEmptyValues)
	}

	var id int
	builder := sq.Insert(obj.Table()).Suffix("RETURNING id").SetMap(data)
	sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return obj, fmt.Errorf("%w: %s, %+v: %w", ErrSQLBuild, sql, args, err)
	}

	scanErr := repo.QueryRow(ctx, sql, args...).Scan(&id)
	if scanErr != nil {
		return obj, fmt.Errorf("%s: %s, %+v: %w", msgCreate, sql, args, scanErr)
	}

	obj.SetID(id)

	return obj, nil
}

// Update updates data in a model table and returns true only when one row changed.
func Update[T any, PT queryModel[T]](
	ctx context.Context,
	repo Repository,
	where map[string]any,
	data map[string]any,
	exprs ...sq.Sqlizer,
) (bool, error) {
	obj := PT(new(T))

	if !hasPredicate(where, exprs) {
		return false, fmt.Errorf("%w: %w", ErrSQLBuild, ErrEmptyPredicate)
	}
	if len(data) == 0 {
		return false, fmt.Errorf("%s: %w", msgUpdate, ErrEmptyValues)
	}

	builder := sq.Update(obj.Table()).SetMap(data)
	if len(where) > 0 {
		builder = builder.Where(where)
	}

	for _, expr := range exprs {
		if expr != nil {
			builder = builder.Where(expr)
		}
	}

	sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return false, fmt.Errorf("%w: %s %+v %+v: %w", ErrSQLBuild, where, data, exprs, err)
	}

	res, err := repo.Exec(ctx, sql, args...)
	if err != nil {
		return false, fmt.Errorf("%s: %s, %+v: %w", msgUpdate, sql, args, err)
	}
	return res.RowsAffected() == 1, nil
}

// Exists checks whether a model row exists for a predicate.
func Exists[T any, PT queryModel[T]](
	ctx context.Context,
	repo Repository,
	where map[string]any,
	exprs ...sq.Sqlizer,
) (bool, error) {
	obj := PT(new(T))

	builder := sq.Select("1").From(obj.Table())
	if len(where) > 0 {
		builder = builder.Where(where)
	}

	for _, expr := range exprs {
		if expr != nil {
			builder = builder.Where(expr)
		}
	}

	sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return false, fmt.Errorf("%w: %s, %+v: %w", ErrSQLBuild, where, exprs, err)
	}

	var res int
	err = repo.QueryRow(ctx, sql, args...).Scan(&res)
	if err != nil {
		if errorsIsNoRows(err) {
			return false, nil
		}
		return res == 1, fmt.Errorf("%s: %s, %+v: %w", msgRetrieveOne, sql, args, err)
	}
	return res == 1, nil
}

// Delete deletes rows from a model table and returns true only when one row changed.
func Delete[T any, PT queryModel[T]](
	ctx context.Context,
	repo Repository,
	where map[string]any,
	exprs ...sq.Sqlizer,
) (bool, error) {
	obj := PT(new(T))

	if !hasPredicate(where, exprs) {
		return false, fmt.Errorf("%w: %w", ErrSQLBuild, ErrEmptyPredicate)
	}

	builder := sq.Delete(obj.Table())
	if len(where) > 0 {
		builder = builder.Where(where)
	}

	for _, expr := range exprs {
		if expr != nil {
			builder = builder.Where(expr)
		}
	}

	sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return false, fmt.Errorf("%w: %s, %+v: %w", ErrSQLBuild, where, exprs, err)
	}

	res, err := repo.Exec(ctx, sql, args...)
	if err != nil {
		return false, fmt.Errorf("%s: %s, %+v: %w", msgDelete, sql, args, err)
	}
	return res.RowsAffected() == 1, nil
}

func errorsIsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func hasPredicate(where map[string]any, exprs []sq.Sqlizer) bool {
	if len(where) > 0 {
		return true
	}

	return slices.ContainsFunc(exprs, isSubstantivePredicate)
}

func isSubstantivePredicate(expr sq.Sqlizer) bool {
	if expr == nil {
		return false
	}

	sql, _, err := expr.ToSql()
	if err != nil {
		return true
	}

	sql = strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(sql)), " "))
	if sql == "" {
		return false
	}

	stripped := trimOuterParens(sql)
	compactStripped := strings.ReplaceAll(stripped, " ", "")

	if stripped == "" || compactStripped == "1=1" || compactStripped == "true" {
		return false
	}

	compact := strings.ReplaceAll(sql, " ", "")
	return compact != "1=1" && compact != "true" &&
		!strings.Contains(compact, "(1=1)") && !strings.Contains(compact, "(true)")
}

func trimOuterParens(sql string) string {
	for len(sql) >= 2 && sql[0] == '(' && sql[len(sql)-1] == ')' && outerParensWrap(sql) {
		sql = strings.TrimSpace(sql[1 : len(sql)-1])
	}
	return sql
}

func outerParensWrap(sql string) bool {
	depth := 0

	for i, r := range sql {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i != len(sql)-1 {
				return false
			}
		}

		if depth < 0 {
			return false
		}
	}
	return depth == 0
}
