package businessevents

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"github.com/global-torque/go-common/queue/v2/pclient"
	"github.com/stretchr/testify/require"
)

func sample(t *testing.T) pclient.Event {
	t.Helper()
	event, err := New(Input{ID: "b62668af-7a23-4d52-a1db-92b91c89a812", Action: "investment.review-recorded.v1", Sender: "investment-api", ObjectName: "investment", ObjectRef: "42", ObjectID: 42, OccurredAt: time.Date(2026, 9, 8, 12, 0, 0, 123456789, time.FixedZone("offset", 3600)), Data: map[string]any{"amount": "123456789012345678.000000000000000001", "nested": []any{nil, true, json.Number("123456789012345678901234567890.123456789")}}})
	require.NoError(t, err)
	return event
}
func TestCanonicalFact(t *testing.T) {
	event := sample(t)
	require.Equal(t, time.UTC, event.OccurredAt.Location())
	require.Equal(t, 123456000, event.OccurredAt.Nanosecond())
	raw, err := Marshal(event)
	require.NoError(t, err)
	require.Contains(t, string(raw), "123456789012345678901234567890.123456789")
	require.NotContains(t, string(raw), "attempt")
	require.NotContains(t, string(raw), "ip_address")
	var legacy map[string]any
	raw, err = json.Marshal(pclient.Event{})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &legacy))
	for _, field := range []string{"version", "occurred_at", "object_ref"} {
		require.NotContains(t, legacy, field)
	}
	require.Contains(t, legacy, "attempt")
	require.Contains(t, legacy, "ip_address")
}
func TestRejectMalformedFact(t *testing.T) {
	for _, value := range []any{math.NaN(), math.Inf(1), struct{ Secret string }{"hidden"}, json.RawMessage(`{"broken":`), json.Number("null")} {
		t.Run("unsupported", func(t *testing.T) {
			event := sample(t)
			event.Data["bad"] = value
			_, err := Marshal(event)
			require.Error(t, err)
		})
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	event := sample(t)
	event.Data = cycle
	require.Error(t, Validate(event))
	event = sample(t)
	event.ObjectRef = "43"
	require.Error(t, Validate(event))
	event = sample(t)
	ts := event.OccurredAt.Add(time.Nanosecond)
	event.OccurredAt = &ts
	require.Error(t, Validate(event))
}
func TestSharedContractFixtures(t *testing.T) {
	raw, err := os.ReadFile("testdata/events.json")
	require.NoError(t, err)
	var fixtures []struct {
		Name  string
		Valid bool
		Event json.RawMessage
	}
	require.NoError(t, json.Unmarshal(raw, &fixtures))
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			var event pclient.Event
			decoder := json.NewDecoder(bytes.NewReader(fixture.Event))
			decoder.UseNumber()
			err := decoder.Decode(&event)
			if err == nil {
				err = Validate(event)
			}
			if fixture.Valid {
				require.NoError(t, err)
				raw, err := Marshal(event)
				require.NoError(t, err)
				require.JSONEq(t, string(fixture.Event), string(raw))
			} else {
				require.Error(t, err)
			}
		})
	}
}
func TestNewSnapshotsCallerData(t *testing.T) {
	original := map[string]any{"nested": map[string]any{"name": "original"}, "number": 42}
	event, err := New(Input{Action: "test.v1", Sender: "test-api", ObjectName: "test", ObjectRef: "42", ObjectID: 42, OccurredAt: time.Now(), Data: original})
	require.NoError(t, err)
	before, err := Marshal(event)
	require.NoError(t, err)
	require.IsType(t, 42, original["number"])
	original["number"] = 43
	original["nested"].(map[string]any)["name"] = "changed"
	after, err := Marshal(event)
	require.NoError(t, err)
	require.Equal(t, before, after)
}
