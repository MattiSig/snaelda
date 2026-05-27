package billing

import (
	"testing"

	"github.com/stripe/stripe-go/v85/webhook"
)

func TestStripeWebhookAcceptsSignedOlderAPIVersionEvent(t *testing.T) {
	const secret = "whsec_test"
	payload := []byte(`{
		"id": "evt_test",
		"object": "event",
		"type": "product.created",
		"api_version": "2025-01-27.acacia",
		"data": {"object": {"id": "prod_test"}}
	}`)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  secret,
	})
	client := NewStripeClient("sk_test_secret", secret)

	event, err := client.ConstructWebhookEvent(payload, signed.Header)
	if err != nil {
		t.Fatalf("construct webhook event: %v", err)
	}
	if event.ID != "evt_test" {
		t.Fatalf("expected event id, got %q", event.ID)
	}
	if event.Type != "product.created" {
		t.Fatalf("expected event type, got %q", event.Type)
	}
}
