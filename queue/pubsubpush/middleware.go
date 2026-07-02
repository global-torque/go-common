package pubsubpush

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/global-torque/go-common/logger/v2"
	"github.com/labstack/echo/v4"
)

const componentName = "pubsubpush"

// MaxAttempts returns Echo middleware that ack-and-drops a Pub/Sub push
// message (HTTP 204) once its DeliveryAttempt exceeds n. Use it as a
// code-level safety net in front of push handlers; the canonical solution
// is to configure max-delivery-attempts + dead-letter-topic on the
// subscription itself, which moves the message to a DLQ topic at
// attempt n+1 instead of letting Pub/Sub retry for the message TTL.
//
// The middleware buffers and rewinds the request body so downstream
// handlers can decode it normally. A request whose body cannot be parsed
// as a Pub/Sub envelope is passed through unchanged — handlers decide how
// to handle malformed deliveries.
func MaxAttempts(n int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			body, err := io.ReadAll(c.Request().Body)
			if err != nil {
				return next(c)
			}
			c.Request().Body = io.NopCloser(bytes.NewReader(body))

			var req PushRequest
			if err := json.Unmarshal(body, &req); err != nil {
				return next(c)
			}

			if req.DeliveryAttempt != nil && *req.DeliveryAttempt > n {
				logger.FromCtx(c.Request().Context(), componentName).Warn().
					Int("delivery_attempt", *req.DeliveryAttempt).
					Int("max_attempts", n).
					Str("message_id", req.Message.MessageID).
					Str("subscription", req.Subscription).
					Bytes("data", req.Message.Data).
					Msg("dropping pubsub push after max delivery attempts")
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}
