package pclient

import (
	"context"
	"errors"
	"testing"
)

func TestHandleEventDeliveryRecordsInvalidPayloadAndReturnsOriginalError(t *testing.T) {
	t.Parallel()

	client := &Client{}
	msg := Message{ID: "message-1", Data: []byte(`{"action":`)}
	var recorded Message
	var validationErr error
	err := client.handleEventDelivery(
		context.Background(),
		msg,
		func(context.Context, Event) error {
			t.Fatal("callback called for invalid event")
			return nil
		},
		func(_ context.Context, got Message, gotErr error) error {
			recorded, validationErr = got, gotErr
			return nil
		},
	)
	if err == nil || validationErr == nil {
		t.Fatalf("errors = (%v, %v), want validation failure", err, validationErr)
	}
	if recorded.ID != msg.ID || string(recorded.Data) != string(msg.Data) {
		t.Fatalf("recorded message = %#v, want %#v", recorded, msg)
	}
}

func TestHandleEventDeliveryReturnsRecorderFailure(t *testing.T) {
	t.Parallel()

	client := &Client{}
	recorderErr := errors.New("database unavailable")
	err := client.handleEventDelivery(
		context.Background(),
		Message{ID: "message-1", Data: []byte(`not-json`)},
		func(context.Context, Event) error { return nil },
		func(context.Context, Message, error) error { return recorderErr },
	)
	if !errors.Is(err, recorderErr) {
		t.Fatalf("error = %v, want recorder error", err)
	}
}
