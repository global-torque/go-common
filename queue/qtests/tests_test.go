//nolint:paralleltest,thelper
package qtests

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/webdevelop-pro/go-common/configurator"
	"github.com/webdevelop-pro/go-common/queue/pclient"
	gTests "github.com/webdevelop-pro/go-common/tests"
)

const pubsubDialTimeout = 500 * time.Millisecond

func requirePubsubIntegration(t *testing.T) {
	t.Helper()

	if err := configurator.LoadDotEnv(); err != nil {
		t.Fatalf("load .env: %v", err)
	}

	if strings.TrimSpace(os.Getenv("PUBSUB_PROJECT_ID")) == "" {
		t.Skip("PUBSUB_PROJECT_ID is required for Pub/Sub integration tests")
	}

	emulatorHost := strings.TrimSpace(os.Getenv("PUBSUB_EMULATOR_HOST"))
	if emulatorHost == "" {
		t.Skip("PUBSUB_EMULATOR_HOST is required for Pub/Sub integration tests")
	}

	conn, err := net.DialTimeout("tcp", emulatorHost, pubsubDialTimeout)
	if err != nil {
		t.Skipf("Pub/Sub emulator is not reachable at %s: %v", emulatorHost, err)
	}
	_ = conn.Close()
}

func uniquePubsubName(t *testing.T, prefix, fallback string) string {
	t.Helper()

	if strings.TrimSpace(prefix) == "" {
		prefix = fallback
	}

	var b strings.Builder
	for _, r := range strings.ToLower(prefix + "_" + t.Name()) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	name := b.String()
	if len(name) > 120 {
		name = name[:120]
	}

	return name + "_" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func TestExample(t *testing.T) {
	requirePubsubIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := uniquePubsubName(t, os.Getenv("PUBSUB_TOPIC_WEBHOOK"), "test_webhooks")
	subscription := uniquePubsubName(t, os.Getenv("PUBSUB_SUBSCRIPTION_WEBHOOK"), topic+"_sub")
	fixtures := NewFixturesManager(ctx, NewFixture(topic, subscription, ""))
	t.Cleanup(func() {
		if err := fixtures.Delete(topic, subscription); err != nil {
			t.Errorf("cleanup Pub/Sub fixture: %v", err)
		}
		fixtures.Close()
	})

	gTests.RunTableTest(t, ctx,
		[]gTests.FixturesManager{
			fixtures,
		},
		gTests.TableTest{
			Description: "test of the table test",
			Scenarios: []gTests.TestScenario{
				{
					Description: "Success test",
					TestActions: []gTests.SomeAction{
						// SendHttpRequst("POST", "/events/sendgrid/test_topic?object=email&action=update&auth_type=auto&auth_token=XXXXX", []byte(`{"test": "message"}`)),
						SendPubSubEvent(topic, "{}", map[string]string{}),
						gTests.Sleep(time.Second * 2),
						SendPubSubEvent(
							topic,
							pclient.Webhook{
								Object:  "profile",
								Action:  "update_accr",
								Service: "north_capital",
								Data: []byte(`
										"accountId":["NO_INVESTMENTS"],
										"airequestId":["Tzboaa"],
										"aiRequestStatus":["Approved"],
										"accreditedStatus":["Verified Accredited"],
									}`),
							},
							map[string]string{},
						),
					},
				},
			},
		},
	)
}
