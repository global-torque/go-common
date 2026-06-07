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

func TestTimestamptzSetCompatibility(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	var ts Timestamptz
	if err := ts.Set(now); err != nil {
		t.Fatalf("Set time returned error: %v", err)
	}
	if !ts.Time.Equal(now) || ts.Status != Present || ts.InfinityModifier != None {
		t.Fatalf("unexpected timestamp: %#v", ts)
	}

	if err := ts.Set((*time.Time)(nil)); err != nil {
		t.Fatalf("Set nil time pointer returned error: %v", err)
	}
	if ts.Status != Null {
		t.Fatalf("expected null, got %#v", ts)
	}

	if err := ts.Set(Infinity); err != nil {
		t.Fatalf("Set infinity returned error: %v", err)
	}
	if ts.Status != Present || ts.InfinityModifier != Infinity {
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

func TestJSONBSetAssignToCompatibility(t *testing.T) {
	payload := []map[string]any{{
		"type": "functions-on-contract",
		"data": map[string]any{
			"address":   "0xdddddddddddddddddddddddddddddddddddddddd",
			"functions": []string{"0x0357371d"},
		},
	}}

	var js JSONB
	if err := js.Set(payload); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if js.Status != Present || !json.Valid(js.Bytes) {
		t.Fatalf("unexpected jsonb: %#v", js)
	}

	var got []map[string]any
	if err := js.AssignTo(&got); err != nil {
		t.Fatalf("AssignTo returned error: %v", err)
	}
	if len(got) != 1 || got[0]["type"] != "functions-on-contract" {
		t.Fatalf("unexpected assigned payload: %#v", got)
	}

	var raw []byte
	if err := js.AssignTo(&raw); err != nil {
		t.Fatalf("AssignTo bytes returned error: %v", err)
	}
	if !reflect.DeepEqual(raw, js.Bytes) {
		t.Fatalf("unexpected raw payload: got %s want %s", raw, js.Bytes)
	}
}

func TestJSONBSetNullCompatibility(t *testing.T) {
	var js JSONB
	if err := js.Set(nil); err != nil {
		t.Fatalf("Set nil returned error: %v", err)
	}
	if js.Status != Null {
		t.Fatalf("expected null, got %#v", js)
	}

	var payload map[string]any
	if err := js.AssignTo(&payload); err != nil {
		t.Fatalf("AssignTo null returned error: %v", err)
	}
	if payload != nil {
		t.Fatalf("expected nil payload, got %#v", payload)
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
