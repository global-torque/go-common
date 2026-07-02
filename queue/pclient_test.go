package queue

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/global-torque/go-common/configurator"
	pclient "github.com/global-torque/go-common/queue/pclient"
)

const (
	pubsubTestTimeout      = 30 * time.Second
	pubsubOperationTimeout = 5 * time.Second
	pubsubDialTimeout      = 500 * time.Millisecond
)

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

func newPubsubTestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	requirePubsubIntegration(t)

	return context.WithTimeout(context.Background(), pubsubTestTimeout)
}

func newPubsubTestClient(t *testing.T, ctx context.Context) *pclient.Client {
	t.Helper()

	pubsubClient, err := pclient.New(ctx)
	if err != nil {
		t.Fatalf("cannot connect: %s", err)
	}
	t.Cleanup(pubsubClient.Close)

	return pubsubClient
}

func pubsubOperationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}

	return context.WithTimeout(parent, pubsubOperationTimeout)
}

func sanitizePubsubNamePart(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
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

	if b.Len() == 0 {
		return "test"
	}

	return b.String()
}

func pubsubTestNames(t *testing.T) (string, string) {
	t.Helper()

	name := sanitizePubsubNamePart(t.Name())
	if len(name) > 120 {
		name = name[:120]
	}

	topicName := fmt.Sprintf("go_common_%s_%s", name, strconv.FormatInt(time.Now().UnixNano(), 36))
	if len(topicName) > 240 {
		topicName = topicName[:240]
	}

	return topicName, topicName + "_sub"
}

func mustNoError(t *testing.T, err error, action string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %s", action, err)
	}
}

func createPubsubTopic(t *testing.T, parent context.Context, pubsubClient *pclient.Client, topicName string) {
	t.Helper()

	ctx, cancel := pubsubOperationContext(parent)
	defer cancel()

	_, err := pubsubClient.CreateTopic(ctx, topicName)
	mustNoError(t, err, "create topic")
}

func createPubsubSubscription(
	t *testing.T,
	parent context.Context,
	pubsubClient *pclient.Client,
	subscriptionName,
	topicName string,
) {
	t.Helper()

	ctx, cancel := pubsubOperationContext(parent)
	defer cancel()

	_, err := pubsubClient.CreateSubscription(ctx, subscriptionName, topicName)
	mustNoError(t, err, "create subscription")
}

func setupPubsub(t *testing.T, ctx context.Context, pubsubClient *pclient.Client, withSubscription bool) (string, string) {
	t.Helper()

	topicName, subscriptionName := pubsubTestNames(t)
	createPubsubTopic(t, ctx, pubsubClient, topicName)

	if withSubscription {
		createPubsubSubscription(t, ctx, pubsubClient, subscriptionName, topicName)
	} else {
		subscriptionName = ""
	}

	t.Cleanup(func() {
		cleanupPubsub(t, pubsubClient, topicName, subscriptionName)
	})

	return topicName, subscriptionName
}

func cleanupPubsub(t *testing.T, pubsubClient *pclient.Client, topicName, subscriptionName string) {
	t.Helper()

	if subscriptionName != "" {
		ctx, cancel := pubsubOperationContext(context.Background())
		err := pubsubClient.DeleteSubscription(ctx, subscriptionName)
		cancel()
		if err != nil {
			t.Errorf("cleanup delete subscription %s: %s", subscriptionName, err)
		}
	}

	ctx, cancel := pubsubOperationContext(context.Background())
	err := pubsubClient.DeleteTopic(ctx, topicName)
	cancel()
	if err != nil {
		t.Errorf("cleanup delete topic %s: %s", topicName, err)
	}
}

func waitUntil(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}

		select {
		case <-deadline.C:
			t.Fatal(message)
		case <-ticker.C:
		}
	}
}

func waitListenerStopped(t *testing.T, errCh <-chan error) {
	t.Helper()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("listener failed: %s", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("listener did not stop after context cancellation")
	}
}

func testEventPayload() pclient.Event {
	return pclient.Event{
		Action:     pclient.PostUpdate,
		Sender:     "go-common-test",
		ObjectID:   123,
		ObjectName: "message",
		RequestID:  "request-id",
		IPAddress:  "127.0.0.1",
		Data: map[string]any{
			"message": 123,
		},
	}
}

func TestPublish(t *testing.T) {
	ctx, cancel := newPubsubTestContext(t)
	defer cancel()

	pubsubClient := newPubsubTestClient(t, ctx)
	topic, _ := setupPubsub(t, ctx, pubsubClient, false)

	t.Run("success publish", func(t *testing.T) {
		opCtx, opCancel := pubsubOperationContext(ctx)
		defer opCancel()

		msg, err := pubsubClient.Publish(opCtx,
			topic,
			map[string]any{"investment_id": 5},
			map[string]string{"ip_address": "31.5.12.199", "request_id": "Xbsdf124d"},
		)
		if err != nil {
			t.Fatalf("errors don't match: expected nil, got %s", err)
		}

		id, err := strconv.Atoi(msg.ID)
		if err != nil {
			t.Errorf("pubsub emulator return ID as int, maybe you  are not using it? %s", err)
		}
		if id <= 0 {
			t.Errorf("msg id should be more that 0 %s", err)
		}
	})
}

func TestListenNack(t *testing.T) {
	var receivedCounter atomic.Int32

	ctx, cancel := newPubsubTestContext(t)
	defer cancel()

	pubsubClient := newPubsubTestClient(t, ctx)
	topic, subscription := setupPubsub(t, ctx, pubsubClient, true)

	t.Run("success nack", func(t *testing.T) {
		listenCtx, stopListen := context.WithCancel(ctx)
		listenerStopped := false
		errCh := make(chan error, 1)
		t.Cleanup(func() {
			if listenerStopped {
				return
			}
			stopListen()
			waitListenerStopped(t, errCh)
		})

		go func() {
			errCh <- pubsubClient.ListenRawMsgs(listenCtx, subscription, topic, func(ctx context.Context, msg pclient.Message) error {
				count := receivedCounter.Add(1)
				if count%2 != 0 {
					return fmt.Errorf("odd number return an error")
				}
				return nil
			})
		}()

		opCtx, opCancel := pubsubOperationContext(ctx)
		_, err := pubsubClient.Publish(opCtx, topic, map[string]int{"message": 123}, map[string]string{})
		opCancel()
		mustNoError(t, err, "publish first message")

		opCtx, opCancel = pubsubOperationContext(ctx)
		_, err = pubsubClient.Publish(opCtx, topic, map[string]int{"message": 123}, map[string]string{})
		opCancel()
		mustNoError(t, err, "publish second message")

		waitUntil(t, 8*time.Second, func() bool {
			return receivedCounter.Load() >= 3
		}, fmt.Sprintf("expected at least 3 receives after nack, got %d", receivedCounter.Load()))

		stopListen()
		waitListenerStopped(t, errCh)
		listenerStopped = true
	})
}

func TestListenAck(t *testing.T) {
	var receivedCounter atomic.Int32

	ctx, cancel := newPubsubTestContext(t)
	defer cancel()

	pubsubClient := newPubsubTestClient(t, ctx)
	topic, subscription := setupPubsub(t, ctx, pubsubClient, true)

	t.Run("success ack", func(t *testing.T) {
		listenCtx, stopListen := context.WithCancel(ctx)
		listenerStopped := false
		errCh := make(chan error, 1)
		t.Cleanup(func() {
			if listenerStopped {
				return
			}
			stopListen()
			waitListenerStopped(t, errCh)
		})

		go func() {
			errCh <- pubsubClient.ListenEvents(listenCtx, subscription, topic, func(ctx context.Context, msg pclient.Event) error {
				receivedCounter.Add(1)
				if msg.ID == "" || msg.Action == "" || msg.ObjectID == 0 || msg.ObjectName == "" {
					return fmt.Errorf("event is empty, its not correct")
				}
				return nil
			})
		}()

		opCtx, opCancel := pubsubOperationContext(ctx)
		_, err := pubsubClient.PublishEvent(opCtx, topic, testEventPayload())
		opCancel()
		mustNoError(t, err, "publish first event")

		opCtx, opCancel = pubsubOperationContext(ctx)
		_, err = pubsubClient.PublishEvent(opCtx, topic, testEventPayload())
		opCancel()
		mustNoError(t, err, "publish second event")

		waitUntil(t, 8*time.Second, func() bool {
			return receivedCounter.Load() >= 2
		}, fmt.Sprintf("expected 2 acknowledged events, got %d", receivedCounter.Load()))

		stopListen()
		waitListenerStopped(t, errCh)
		listenerStopped = true
	})
}

func TestReconnectToNonExistTopic(t *testing.T) {
	var receivedCounter atomic.Int32

	ctx, cancel := newPubsubTestContext(t)
	defer cancel()

	pubsubClient := newPubsubTestClient(t, ctx)
	topic, subscription := pubsubTestNames(t)
	t.Cleanup(func() {
		cleanupPubsub(t, pubsubClient, topic, subscription)
	})

	t.Run("success reconnect", func(t *testing.T) {
		listenCtx, stopListen := context.WithCancel(ctx)
		listenerStopped := false
		errCh := make(chan error, 1)
		t.Cleanup(func() {
			if listenerStopped {
				return
			}
			stopListen()
			waitListenerStopped(t, errCh)
		})

		go func() {
			errCh <- pubsubClient.ListenEvents(listenCtx, subscription, topic, func(ctx context.Context, msg pclient.Event) error {
				receivedCounter.Add(1)
				if msg.ID == "" || msg.Action == "" || msg.ObjectID == 0 || msg.ObjectName == "" {
					return fmt.Errorf("event is empty, its not correct")
				}
				return nil
			})
		}()
		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("cannot connect: %s", err)
			}
		case <-time.After(time.Second):
		}

		time.Sleep(time.Second * 5)
		createPubsubTopic(t, ctx, pubsubClient, topic)
		createPubsubSubscription(t, ctx, pubsubClient, subscription, topic)

		opCtx, opCancel := pubsubOperationContext(ctx)
		_, err := pubsubClient.PublishEvent(opCtx, topic, testEventPayload())
		opCancel()
		mustNoError(t, err, "publish first event")

		opCtx, opCancel = pubsubOperationContext(ctx)
		_, err = pubsubClient.PublishEvent(opCtx, topic, testEventPayload())
		opCancel()
		mustNoError(t, err, "publish second event")

		waitUntil(t, 8*time.Second, func() bool {
			return receivedCounter.Load() >= 2
		}, fmt.Sprintf("expected 2 events after reconnect, got %d", receivedCounter.Load()))

		stopListen()
		waitListenerStopped(t, errCh)
		listenerStopped = true
	})
}
