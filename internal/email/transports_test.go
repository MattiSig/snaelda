package email

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewMailerRequiresResendAPIKey(t *testing.T) {
	if _, err := NewMailer(Config{
		Transport:   "resend",
		DefaultFrom: Address{Email: "hi@snaelda.app"},
	}); err == nil {
		t.Fatal("expected resend api key error")
	}
}

func TestStdoutMailerAppliesDefaultFrom(t *testing.T) {
	mailer, err := NewMailer(Config{
		Transport:   "stdout",
		DefaultFrom: Address{Email: "hi@snaelda.app", Name: "Snaelda"},
		Logger:      slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		Now:         func() time.Time { return time.Unix(123, 0).UTC() },
	})
	if err != nil {
		t.Fatalf("new mailer: %v", err)
	}
	result, err := mailer.Send(context.Background(), Message{
		To:       []Address{{Email: "demo@example.com"}},
		Subject:  "Hello",
		TextBody: "Test",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.HasPrefix(result.ProviderMessageID, "stdout-") {
		t.Fatalf("unexpected provider message id %q", result.ProviderMessageID)
	}
}

func TestResendMailerSendsExpectedPayload(t *testing.T) {
	var gotAuth string
	var gotIdempotency string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotIdempotency = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"re_123"}`))
	}))
	defer server.Close()

	client := server.Client()
	client.Transport = rewriteHostTransport{base: client.Transport, url: server.URL}

	mailer, err := NewMailer(Config{
		Transport:    "resend",
		DefaultFrom:  Address{Email: "hi@snaelda.app", Name: "Snaelda"},
		ResendAPIKey: "test-key",
		HTTPClient:   client,
		Now:          func() time.Time { return time.Unix(123, 0).UTC() },
	})
	if err != nil {
		t.Fatalf("new mailer: %v", err)
	}

	result, err := mailer.Send(context.Background(), Message{
		To:             []Address{{Email: "demo@example.com", Name: "Demo"}},
		Subject:        "Magic",
		TextBody:       "Text",
		HTMLBody:       "<p>Text</p>",
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if result.ProviderMessageID != "re_123" {
		t.Fatalf("expected resend id, got %q", result.ProviderMessageID)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("expected auth header, got %q", gotAuth)
	}
	if gotIdempotency != "idem-1" {
		t.Fatalf("expected idempotency header, got %q", gotIdempotency)
	}
	if gotBody["subject"] != "Magic" {
		t.Fatalf("expected subject in payload, got %#v", gotBody["subject"])
	}
}

func TestMemoryMailerCapturesMessages(t *testing.T) {
	mailer := NewMemoryMailer()
	if _, err := mailer.Send(context.Background(), Message{
		To:       []Address{{Email: "demo@example.com"}},
		Subject:  "Hello",
		TextBody: "Test",
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(mailer.Messages) != 1 {
		t.Fatalf("expected one message, got %d", len(mailer.Messages))
	}
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}

type rewriteHostTransport struct {
	base http.RoundTripper
	url  string
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	replacement := strings.TrimSuffix(t.url, "/")
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(replacement, "http://")
	if strings.HasPrefix(replacement, "https://") {
		req.URL.Scheme = "https"
		req.URL.Host = strings.TrimPrefix(replacement, "https://")
	}
	return t.base.RoundTrip(req)
}
