package pgtype

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	pgxpgtype "github.com/jackc/pgx/v5/pgtype"
)

func TestTextPgxInterfaces(t *testing.T) {
	var text Text
	if err := text.ScanText(pgxpgtype.Text{String: "hello", Valid: true}); err != nil {
		t.Fatalf("ScanText returned error: %v", err)
	}
	if text.String != "hello" || text.Status != Present {
		t.Fatalf("unexpected text: %#v", text)
	}

	value, err := text.TextValue()
	if err != nil {
		t.Fatalf("TextValue returned error: %v", err)
	}
	if value.String != "hello" || !value.Valid {
		t.Fatalf("unexpected value: %#v", value)
	}

	if err := text.ScanText(pgxpgtype.Text{}); err != nil {
		t.Fatalf("ScanText null returned error: %v", err)
	}
	if text.Status != Null {
		t.Fatalf("expected null, got %#v", text)
	}
}

func TestTimestamptzPgxInterfaces(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	var ts Timestamptz
	if err := ts.ScanTimestamptz(pgxpgtype.Timestamptz{Time: now, Valid: true}); err != nil {
		t.Fatalf("ScanTimestamptz returned error: %v", err)
	}
	if !ts.Time.Equal(now) || ts.Status != Present || ts.InfinityModifier != None {
		t.Fatalf("unexpected timestamp: %#v", ts)
	}

	value, err := ts.TimestamptzValue()
	if err != nil {
		t.Fatalf("TimestamptzValue returned error: %v", err)
	}
	if !value.Time.Equal(now) || !value.Valid || value.InfinityModifier != pgxpgtype.Finite {
		t.Fatalf("unexpected value: %#v", value)
	}

	if err := ts.ScanTimestamptz(pgxpgtype.Timestamptz{Valid: true, InfinityModifier: pgxpgtype.Infinity}); err != nil {
		t.Fatalf("ScanTimestamptz infinity returned error: %v", err)
	}
	if ts.InfinityModifier != Infinity {
		t.Fatalf("expected infinity, got %#v", ts)
	}
}

func TestJSONPgxInterfaces(t *testing.T) {
	var js JSON
	if err := js.ScanBytes([]byte(`{"a":1}`)); err != nil {
		t.Fatalf("ScanBytes returned error: %v", err)
	}
	if js.Status != Present || !reflect.DeepEqual(js.Bytes, []byte(`{"a":1}`)) {
		t.Fatalf("unexpected json: %#v", js)
	}

	value, err := js.BytesValue()
	if err != nil {
		t.Fatalf("BytesValue returned error: %v", err)
	}
	if !reflect.DeepEqual(value, []byte(`{"a":1}`)) {
		t.Fatalf("unexpected value: %#v", value)
	}

	if err := js.ScanBytes(nil); err != nil {
		t.Fatalf("ScanBytes null returned error: %v", err)
	}
	if js.Status != Null {
		t.Fatalf("expected null, got %#v", js)
	}
}

func TestJSONBPgxInterfaces(t *testing.T) {
	var js JSONB
	if err := js.ScanBytes([]byte(`{"a":1}`)); err != nil {
		t.Fatalf("ScanBytes returned error: %v", err)
	}
	if js.Status != Present || !reflect.DeepEqual(js.Bytes, []byte(`{"a":1}`)) {
		t.Fatalf("unexpected jsonb: %#v", js)
	}
}

func TestJSONMarshalUsesRawPayload(t *testing.T) {
	value, err := json.Marshal(JSON{Bytes: []byte(`{"a":1}`), Status: Present})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if string(value) != `{"a":1}` {
		t.Fatalf("unexpected JSON: %s", value)
	}

	value, err = json.Marshal(JSONB{Bytes: []byte(`{"b":2}`), Status: Present})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if string(value) != `{"b":2}` {
		t.Fatalf("unexpected JSONB: %s", value)
	}
}

func TestUndefinedValuesReturnError(t *testing.T) {
	_, err := Text{}.TextValue()
	if !errors.Is(err, errUndefined) {
		t.Fatalf("expected undefined error, got %v", err)
	}
	_, err = Timestamptz{}.TimestamptzValue()
	if !errors.Is(err, errUndefined) {
		t.Fatalf("expected undefined error, got %v", err)
	}
	_, err = JSON{}.BytesValue()
	if !errors.Is(err, errUndefined) {
		t.Fatalf("expected undefined error, got %v", err)
	}
}
