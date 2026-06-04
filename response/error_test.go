package response

import "testing"

func TestErrorNilSafe(t *testing.T) {
	err := NewError(nil)

	if got := err.Error(); got != "" {
		t.Fatalf("expected empty error string for nil wrapped error, got %q", got)
	}
	if got := err.Unwrap(); got == nil {
		t.Fatalf("expected nil wrapped error to be normalized")
	}
}

func TestErrorMessageMapSafe(t *testing.T) {
	err := NewError(nil, "plain text")

	if _, ok := err.GetMessageFromMap("__error__"); !ok {
		t.Fatalf("expected string argument to be exposed as message map")
	}

	err.AddMessageToMap("field", "message")

	values, ok := err.GetMessageFromMap("field")
	if !ok {
		t.Fatalf("expected message to be added to map")
	}
	if len(values) != 1 || values[0] != "message" {
		t.Fatalf("expected added message, got %#v", values)
	}
}

func TestErrorMessagesAreCopied(t *testing.T) {
	rawMessages := map[string][]string{"field": {"original"}}
	messages := NewErrorMessages(rawMessages)
	err := NewError(nil, messages)
	rawMessages["field"][0] = "mutated"

	values, ok := err.GetMessageFromMap("field")
	if !ok {
		t.Fatalf("expected field message")
	}
	if values[0] != "original" {
		t.Fatalf("expected copied message, got %q", values[0])
	}

	values[0] = "changed"
	values, _ = err.GetMessageFromMap("field")
	if values[0] != "original" {
		t.Fatalf("expected returned values to be copied, got %q", values[0])
	}
}

func TestErrorMessagesMapIsCopied(t *testing.T) {
	err := NewError(nil, map[string][]string{"field": {"original"}})
	values := err.Message.Map()
	values["field"][0] = "changed"

	values = err.Message.Map()
	if values["field"][0] != "original" {
		t.Fatalf("expected message map to be immutable from caller changes, got %q", values["field"][0])
	}
}
