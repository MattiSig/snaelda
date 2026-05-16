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
