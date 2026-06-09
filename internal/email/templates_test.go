package email

import (
	"strings"
	"testing"
)

func TestRenderMagicLinkLogin(t *testing.T) {
	subject, textBody, htmlBody, err := RenderMagicLinkLogin(MagicLinkTemplateData{
		ProductName: "Snaelda",
		ActionLabel: "Log in",
		MagicURL:    "https://example.test/magic",
		ExpiresIn:   "15 minutes",
	})
	if err != nil {
		t.Fatalf("render magic link login: %v", err)
	}
	if subject != "Your Snaelda login link" {
		t.Fatalf("unexpected subject %q", subject)
	}
	if !strings.Contains(textBody, "https://example.test/magic") {
		t.Fatal("expected magic url in text body")
	}
	if !strings.Contains(htmlBody, `href="https://example.test/magic"`) {
		t.Fatal("expected magic url in html body")
	}
}

func TestRenderFormSubmissionForwarded(t *testing.T) {
	_, textBody, htmlBody, err := RenderFormSubmissionForwarded(FormSubmissionForwardedTemplateData{
		SiteName:    "Ribbon Studio",
		PageTitle:   "Contact",
		SubmittedAt: "2026-05-16T12:00:00Z",
		Fields: []ForwardedField{
			{Label: "Name", Value: "Ada"},
			{Label: "Message", Value: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("render form forwarding: %v", err)
	}
	if !strings.Contains(textBody, "Name: Ada") {
		t.Fatal("expected text field rendering")
	}
	if !strings.Contains(htmlBody, "<strong>Message:</strong> Hello") {
		t.Fatal("expected html field rendering")
	}
}

func TestRenderOnceOverDelivered(t *testing.T) {
	_, textBody, htmlBody, err := RenderOnceOverDelivered(OnceOverDeliveredTemplateData{
		ProductName:   "Snaelda",
		WorkspaceName: "Wool Shop",
		DeliveryURL:   "https://loom.test/share/123",
		NextSteps: []string{
			"Replace the last placeholder photo.",
			"Connect the custom domain.",
		},
	})
	if err != nil {
		t.Fatalf("render once-over delivered: %v", err)
	}
	if !strings.Contains(textBody, "Replace the last placeholder photo.") {
		t.Fatal("expected next steps in text body")
	}
	if !strings.Contains(htmlBody, "<li>Connect the custom domain.</li>") {
		t.Fatal("expected next steps in html body")
	}
}
