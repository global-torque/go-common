package pclient

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	PreAdd     EventType = "pre_add"
	PostAdd    EventType = "post_add"
	PreRemove  EventType = "pre_remove"
	PostRemove EventType = "post_remove"
	PreUpdate  EventType = "pre_update"
	PostUpdate EventType = "post_update"
)

type Message struct {
	Headers     map[string][]string
	Attributes  map[string]string
	OrderingKey string
	PublishTime time.Time
	Data        []byte
	Attempt     *int   `json:"attempt"`
	ID          string `json:"message_id"`
}

func NewMessage(data any, attributes map[string]string) (*Message, error) {
	// Returns the message object
	b, err := json.Marshal(&data)
	if err != nil {
		return nil, err
	}
	return &Message{
		Data:       b,
		Attributes: attributes,
	}, nil
}

// func (msg *Message) PubSubAttributes() map[string]string {
// 	pubsubAttrs := map[string]string{}
// 	for k, v := range msg.Attributes {
// 		switch val := v.(type) {
// 		case string:
// 			pubsubAttrs[k] = val
// 		case int:
// 			pubsubAttrs[k] = fmt.Sprintf("%d", val)
// 		case bool:
// 			pubsubAttrs[k] = fmt.Sprintf("%x", val)
// 		case float64:
// 			pubsubAttrs[k] = fmt.Sprintf("%.2f", val)
// 		default:
// 			bData, _ := json.Marshal(val)
// 			pubsubAttrs[k] = string(bData)
// 		}
// 	}
// 	return pubsubAttrs
//}

type Event struct {
	Version    int            `json:"version,omitempty"`
	OccurredAt *time.Time     `json:"occurred_at,omitempty"`
	ObjectRef  string         `json:"object_ref,omitempty"`
	ID         string         `json:"id"`
	Attempt    *int           `json:"attempt"`
	Action     EventType      `json:"action" validate:"required"`
	Sender     string         `json:"sender" validate:"required"`
	ObjectID   int            `json:"object_id" validate:"required"`
	ObjectName string         `json:"object_name" validate:"required"`
	RequestID  string         `json:"request_id"`
	IPAddress  string         `json:"ip_address"`
	Data       map[string]any `json:"data"`
}

type Webhook struct {
	ID      string              `json:"id"`
	Attempt *int                `json:"attempt"`
	Action  string              `json:"action" validate:"required"`
	Object  string              `json:"object" validate:"required"`
	Service string              `json:"service" validate:"required"`
	Headers map[string][]string `json:"headers"`
	Data    []byte              `json:"data"`
}
