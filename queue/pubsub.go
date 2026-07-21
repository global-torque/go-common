package queue

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/webdevelop-pro/go-common/logger"
	"github.com/webdevelop-pro/go-common/queue/pclient"
)

const (
	// reconnectWindow is how long a listener may stay continuously broken
	// (e.g. its subscription was deleted) while attempting to reconnect
	// before we give up and panic so the supervisor restarts the process.
	// A permanently-dead subscription stays dead, so this only governs how
	// fast we surface it; PUBSUB_RECONNECT_WINDOW overrides it (e.g. the
	// end-to-end test harness churns subscriptions and runs one long-lived
	// worker across the whole suite, where 60s is too tight).
	reconnectWindow = 60 * time.Second
	// reconnectDelay paces reconnect attempts so a hard failure does not
	// busy-loop the CPU and flood the logs.
	reconnectDelay = 2 * time.Second
	// healthyRunTime is how long a listener must have stayed connected for a
	// subsequent failure to count as a transient drop (window reset) rather
	// than part of a continuous outage.
	healthyRunTime = 15 * time.Second
)

// Deduper persists per-(service,message) processing state so that a Pub/Sub
// message redelivered (at-least-once / ack-deadline expiry) or published more
// than once is processed at-most-once in effect. It is intentionally defined
// with primitive parameters only — the queue package stays DB-agnostic; the
// service injects an implementation backed by its own storage.
//
// Claim returns claimed=true when the caller now owns processing of msgID
// (first sight, or a retry of a previously failed attempt). claimed=false
// means another delivery is in-flight or it was already processed — the caller
// should ack and skip. A non-nil error should be treated as retryable (NACK).
type Deduper interface {
	Claim(ctx context.Context, service, topic, msgID string, attempt int) (claimed bool, err error)
	MarkProcessed(ctx context.Context, service, msgID string) error
	MarkFailed(ctx context.Context, service, msgID, lastErr string) error
}

type PubSubListener struct {
	log     logger.Logger
	routes  []PubSubRoute
	client  *pclient.Client
	deduper Deduper
	service string
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type PubSubRoute struct {
	// ToDo
	// enum: event, webhook, raw
	Topic            string
	Name             string
	Subscription     string
	WebhooksListener func(ctx context.Context, msg pclient.Webhook) error
	EventsListener   func(ctx context.Context, msg pclient.Event) error
	// InvalidEventsRecorder audits pull deliveries that cannot be decoded as
	// pclient.Event. The delivery remains NACKed after the hook returns.
	InvalidEventsRecorder pclient.InvalidEventRecorder
	MsgsListener          func(ctx context.Context, msg pclient.Message) error
}

func New(routes []PubSubRoute) (*PubSubListener, error) {
	return newListener(routes, "", nil)
}

// MustNew is New with fatal-on-error semantics for app main packages.
func MustNew(routes []PubSubRoute) *PubSubListener {
	listener, err := New(routes)
	if err != nil {
		log := logger.NewComponentLogger(context.TODO(), "pubsub")
		log.Fatal().Err(err).Msg("failed to create pubsub listener")
	}

	return listener
}

// NewWithDeduper is New plus message-level deduplication. service identifies
// this consumer in the dedup store (one row per service+message); d persists
// the claim/processed/failed state. A nil d behaves exactly like New.
func NewWithDeduper(routes []PubSubRoute, service string, d Deduper) (*PubSubListener, error) {
	return newListener(routes, service, d)
}

// MustNewWithDeduper is NewWithDeduper with fatal-on-error semantics for app main packages.
func MustNewWithDeduper(routes []PubSubRoute, service string, d Deduper) *PubSubListener {
	listener, err := NewWithDeduper(routes, service, d)
	if err != nil {
		log := logger.NewComponentLogger(context.TODO(), "pubsub")
		log.Fatal().Err(err).Msg("failed to create pubsub listener")
	}

	return listener
}

func newListener(routes []PubSubRoute, service string, d Deduper) (*PubSubListener, error) {
	log := logger.NewComponentLogger(context.TODO(), "pubsub")

	client, err := pclient.New(context.Background())
	if err != nil {
		return nil, fmt.Errorf("create pubsub client: %w", err)
	}

	p := &PubSubListener{
		log:     log,
		client:  client,
		deduper: d,
		service: service,
	}

	err = p.AddRoutes(routes)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("%w: %w", ErrAddRoute, err)
	}

	return p, nil
}

func (p *PubSubListener) Start(ctx context.Context) error {
	if p == nil || p.client == nil {
		return pclient.ErrNotConnected
	}
	if ctx == nil {
		ctx = context.Background()
	}

	for _, route := range p.routes {
		switch route.Name {
		case "webhooks", "events", "messages":
		default:
			return fmt.Errorf("%w: %s", ErrNotCorrectTopic, route.Name)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	for _, b := range p.routes {
		br := b

		switch br.Name {
		case "webhooks":
			cb := p.dedupWebhooks(br.Topic, br.WebhooksListener)
			p.wg.Add(1)
			go func() {
				defer p.wg.Done()
				p.runListener(ctx, br.Name, br.Subscription, func(ctx context.Context) error {
					return p.client.ListenWebhooks(ctx, br.Subscription, br.Topic, cb)
				})
			}()
		case "events":
			cb := p.dedupEvents(br.Topic, br.EventsListener)
			p.wg.Add(1)
			go func() {
				defer p.wg.Done()
				p.runListener(ctx, br.Name, br.Subscription, func(ctx context.Context) error {
					return p.client.ListenEventsWithInvalidRecorder(
						ctx, br.Subscription, br.Topic, cb, br.InvalidEventsRecorder,
					)
				})
			}()
		case "messages":
			p.wg.Add(1)
			go func() {
				defer p.wg.Done()
				p.runListener(ctx, br.Name, br.Subscription, func(ctx context.Context) error {
					return p.client.ListenRawMsgs(ctx, br.Subscription, br.Topic, br.MsgsListener)
				})
			}()
		}
	}

	return nil
}

func (p *PubSubListener) Close() {
	if p == nil {
		return
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	if p.client != nil {
		p.client.Close()
	}
}

// reconnectWindowDur is the effective reconnect-or-panic window: the
// PUBSUB_RECONNECT_WINDOW env override (a Go duration like "5m") if set and
// valid, otherwise the reconnectWindow default.
func reconnectWindowDur() time.Duration {
	if v := os.Getenv("PUBSUB_RECONNECT_WINDOW"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return reconnectWindow
}

// runListener keeps a subscription listener alive across disconnects.
//
// pclient already retries connecting for up to pclient.SubscriptionRetryTimeout
// (so a subscription deleted and recreated within that window reconnects
// transparently). If the listener cannot stay connected for a continuous
// reconnectWindow — e.g. the subscription was deleted and never recreated —
// runListener panics so the process is restarted by its supervisor instead of
// silently busy-looping forever.
func (p *PubSubListener) runListener(ctx context.Context, name, subscription string, listen func(context.Context) error) {
	var failingSince time.Time

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		start := time.Now()
		err := listen(ctx)
		if err == nil {
			// Graceful stop (context cancelled).
			return
		}
		if ctx.Err() != nil {
			return
		}

		// A listener that stayed up for a while before failing is a transient
		// drop, not a missing subscription: restart the outage window.
		if time.Since(start) >= healthyRunTime {
			failingSince = time.Time{}
		}
		if failingSince.IsZero() {
			failingSince = start
		}

		downFor := time.Since(failingSince)
		if downFor >= reconnectWindowDur() {
			p.log.Error().Err(err).
				Str("subscription", subscription).
				Dur("down_for", downFor).
				Msg(ErrListeningError.Error())
			panic(fmt.Sprintf(
				"pubsub %q listener could not reconnect to subscription %q within %s: %v",
				name, subscription, reconnectWindowDur(), err,
			))
		}

		p.log.Warn().Err(err).
			Str("subscription", subscription).
			Dur("down_for", downFor).
			Msg("pubsub listener disconnected, reconnecting")
		select {
		case <-time.After(reconnectDelay):
		case <-ctx.Done():
			return
		}
	}
}

func (p *PubSubListener) AddRoutes(routes []PubSubRoute) error {
	p.routes = append(p.routes, routes...)

	return nil
}

func attemptOf(a *int) int {
	if a == nil {
		return 0
	}
	return *a
}

// dedupEvents wraps an events callback with claim→handle→finalize. When no
// deduper is configured it returns the original callback unchanged.
func (p *PubSubListener) dedupEvents(
	topic string,
	fn func(context.Context, pclient.Event) error,
) func(context.Context, pclient.Event) error {
	if p.deduper == nil || fn == nil {
		return fn
	}
	return func(ctx context.Context, msg pclient.Event) error {
		claimed, err := p.deduper.Claim(ctx, p.service, topic, msg.ID, attemptOf(msg.Attempt))
		if err != nil {
			return err // retryable: NACK
		}
		if !claimed {
			// already processed, or another delivery is in-flight — skip & ack
			return nil
		}
		if herr := fn(ctx, msg); herr != nil {
			if merr := p.deduper.MarkFailed(ctx, p.service, msg.ID, herr.Error()); merr != nil {
				p.log.Error().Err(merr).Str("msg_id", msg.ID).Msg("cannot mark message failed")
			}
			return herr // NACK; redelivery re-claims the failed row
		}
		return p.deduper.MarkProcessed(ctx, p.service, msg.ID)
	}
}

// dedupWebhooks is the webhook-payload counterpart of dedupEvents.
func (p *PubSubListener) dedupWebhooks(
	topic string,
	fn func(context.Context, pclient.Webhook) error,
) func(context.Context, pclient.Webhook) error {
	if p.deduper == nil || fn == nil {
		return fn
	}
	return func(ctx context.Context, msg pclient.Webhook) error {
		claimed, err := p.deduper.Claim(ctx, p.service, topic, msg.ID, attemptOf(msg.Attempt))
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
		if herr := fn(ctx, msg); herr != nil {
			if merr := p.deduper.MarkFailed(ctx, p.service, msg.ID, herr.Error()); merr != nil {
				p.log.Error().Err(merr).Str("msg_id", msg.ID).Msg("cannot mark message failed")
			}
			return herr
		}
		return p.deduper.MarkProcessed(ctx, p.service, msg.ID)
	}
}
