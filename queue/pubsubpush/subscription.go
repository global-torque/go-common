package pubsubpush

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnexpectedSubscription identifies a push envelope delivered for any
// subscription other than the consumer's exact configured full resource.
var ErrUnexpectedSubscription = errors.New("unexpected Pub/Sub subscription")

// ErrInvalidSubscriptionResource identifies invalid configured project or
// subscription IDs used to build a full resource name.
var ErrInvalidSubscriptionResource = errors.New("invalid Pub/Sub subscription resource")

// SubscriptionResource builds the full Pub/Sub resource used in push
// envelopes from configured project and subscription IDs.
func SubscriptionResource(projectID, subscriptionID string) (string, error) {
	projectID = strings.TrimSpace(projectID)
	subscriptionID = strings.TrimSpace(subscriptionID)

	if projectID == "" || strings.Contains(projectID, "/") {
		return "", fmt.Errorf("%w: project ID %q", ErrInvalidSubscriptionResource, projectID)
	}

	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		return "", fmt.Errorf("%w: subscription ID %q", ErrInvalidSubscriptionResource, subscriptionID)
	}

	return fmt.Sprintf("projects/%s/subscriptions/%s", projectID, subscriptionID), nil
}

// ValidateSubscription requires exact equality with a full configured Pub/Sub
// subscription resource. Partial names and resources from another project are
// rejected.
func ValidateSubscription(actual, expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return fmt.Errorf("%w: expected subscription is empty", ErrUnexpectedSubscription)
	}

	if actual != expected {
		return fmt.Errorf("%w: got %q, want %q", ErrUnexpectedSubscription, actual, expected)
	}

	return nil
}
