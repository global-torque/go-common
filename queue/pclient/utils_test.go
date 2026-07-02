package pclient

import (
	"context"
	"testing"

	"github.com/global-torque/go-common/context/v2/keys"
	"github.com/stretchr/testify/assert"
)

func TestPubSubWebhook(t *testing.T) {
	ctx := context.Background()
	webhook := Webhook{
		Headers: map[string][]string{
			"X-Request-Id":    {"ZXCasdf123"},
			"X-Forwarded-For": {"31.6.1.12"},
		},
		ID: "1",
	}

	ctx = SetDefaultWebhookCtx(ctx, webhook)

	assert.Equal(t, webhook.Headers["X-Request-Id"][0], keys.GetCtxValue(ctx, keys.RequestID))
	assert.Equal(t, webhook.Headers["X-Forwarded-For"][0], keys.GetCtxValue(ctx, keys.IPAddress))
	assert.Equal(t, webhook.ID, keys.GetCtxValue(ctx, keys.MSGID))
}
