package orm

import (
	"reflect"
	"testing"

	sq "github.com/Masterminds/squirrel"
)

func TestChangeSet(t *testing.T) {
	var changes ChangeSet
	changes.Set("name", "alice")
	changes.Set("name", "bob")
	expr := sq.Expr("balance + ?", 10)
	changes.Set("balance", expr)

	got := changes.Changes()
	if !reflect.DeepEqual(got, map[string]any{"name": "bob", "balance": expr}) {
		t.Fatalf("unexpected changes: %#v", got)
	}

	got["name"] = "mallory"
	if changes.Changes()["name"] != "bob" {
		t.Fatalf("Changes did not return a defensive copy")
	}

	changes.ResetChanges()
	if len(changes.Changes()) != 0 {
		t.Fatalf("expected reset changes")
	}
}
