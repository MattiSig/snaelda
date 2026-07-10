package email

import (
	"strings"
	"testing"
)

func TestRenderMagicLinkLogin(t *testing.T) {
	subject, textBody, htmlBody, err := RenderMagicLinkLogin("en", MagicLinkTemplateData{
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

func TestRenderMagicLinkLoginIcelandic(t *testing.T) {
	subject, textBody, htmlBody, err := RenderMagicLinkLogin("is", MagicLinkTemplateData{
		ProductName: "Snaelda",
		ActionLabel: "Skrá inn",
		MagicURL:    "https://example.test/magic",
		ExpiresIn:   "15 mínútum",
	})
	if err != nil {
		t.Fatalf("render magic link login (is): %v", err)
	}
	if subject != "Innskráningarhlekkurinn þinn hjá Snaelda" {
		t.Fatalf("unexpected icelandic subject %q", subject)
	}
	if !strings.Contains(textBody, "skrá þig inn") {
		t.Fatalf("expected icelandic body, got %q", textBody)
	}
	if !strings.Contains(htmlBody, `href="https://example.test/magic"`) {
		t.Fatal("expected magic url in icelandic html body")
	}
}

func TestRenderFormSubmissionForwarded(t *testing.T) {
	_, textBody, htmlBody, err := RenderFormSubmissionForwarded("en", FormSubmissionForwardedTemplateData{
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
	_, textBody, htmlBody, err := RenderOnceOverDelivered("en", OnceOverDeliveredTemplateData{
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

// renderCase renders every template in one locale, exercising every code path
// so the per-locale parity test can assert non-empty subject/text/HTML for all.
func renderAllTemplates(t *testing.T, locale string) {
	t.Helper()

	cases := []struct {
		name   string
		render func() (string, string, string, error)
	}{
		{"magic_link_login", func() (string, string, string, error) {
			return RenderMagicLinkLogin(locale, MagicLinkTemplateData{ProductName: "Snaelda", ActionLabel: "Go", MagicURL: "https://x.test/m", ExpiresIn: "15"})
		}},
		{"magic_link_verify", func() (string, string, string, error) {
			return RenderMagicLinkVerify(locale, MagicLinkTemplateData{ProductName: "Snaelda", ActionLabel: "Go", MagicURL: "https://x.test/m", ExpiresIn: "15"})
		}},
		{"billing_receipt", func() (string, string, string, error) {
			return RenderBillingReceipt(locale, BillingReceiptTemplateData{ProductName: "Snaelda", WorkspaceName: "W", Amount: "2.900", Currency: "ISK", ReceiptURL: "https://x.test/r", PlanName: "Site"})
		}},
		{"billing_payment_failed", func() (string, string, string, error) {
			return RenderBillingPaymentFailed(locale, BillingPaymentFailedTemplateData{ProductName: "Snaelda", WorkspaceName: "W", PlanName: "Site", PortalURL: "https://x.test/p"})
		}},
		{"once_over_intake_ready", func() (string, string, string, error) {
			return RenderOnceOverIntakeReady(locale, OnceOverIntakeReadyTemplateData{ProductName: "Snaelda", WorkspaceName: "W", IntakeURL: "https://x.test/i"})
		}},
		{"once_over_delivered", func() (string, string, string, error) {
			return RenderOnceOverDelivered(locale, OnceOverDeliveredTemplateData{ProductName: "Snaelda", WorkspaceName: "W", DeliveryURL: "https://x.test/d", NextSteps: []string{"One"}})
		}},
		{"form_submission_forwarded", func() (string, string, string, error) {
			return RenderFormSubmissionForwarded(locale, FormSubmissionForwardedTemplateData{ProductName: "Snaelda", SiteName: "S", PageTitle: "Contact", SubmittedAt: "now", Fields: []ForwardedField{{Label: "Name", Value: "Ada"}}})
		}},
		{"workspace_claimed", func() (string, string, string, error) {
			return RenderWorkspaceClaimed(locale, WorkspaceClaimedTemplateData{ProductName: "Snaelda", WorkspaceName: "W", LoginURL: "https://x.test/l"})
		}},
	}

	for _, tc := range cases {
		subject, textBody, htmlBody, err := tc.render()
		if err != nil {
			t.Fatalf("[%s] render %s: %v", locale, tc.name, err)
		}
		if strings.TrimSpace(subject) == "" {
			t.Fatalf("[%s] %s: empty subject", locale, tc.name)
		}
		if strings.TrimSpace(textBody) == "" {
			t.Fatalf("[%s] %s: empty text body", locale, tc.name)
		}
		if strings.TrimSpace(htmlBody) == "" {
			t.Fatalf("[%s] %s: empty html body", locale, tc.name)
		}
		if !strings.Contains(htmlBody, "<html>") {
			t.Fatalf("[%s] %s: html body missing <html> wrapper", locale, tc.name)
		}
	}
}

func TestTemplateLocaleParity(t *testing.T) {
	for _, locale := range SupportedLocales {
		renderAllTemplates(t, locale)
	}
}

func TestUnsupportedLocaleFallsBackToEnglish(t *testing.T) {
	subject, textBody, _, err := RenderMagicLinkLogin("fr", MagicLinkTemplateData{
		ProductName: "Snaelda",
		ActionLabel: "Log in",
		MagicURL:    "https://example.test/magic",
		ExpiresIn:   "15 minutes",
	})
	if err != nil {
		t.Fatalf("render unsupported locale: %v", err)
	}
	if subject != "Your Snaelda login link" {
		t.Fatalf("expected english fallback subject, got %q", subject)
	}
	if !strings.Contains(textBody, "log in to Snaelda") {
		t.Fatalf("expected english fallback body, got %q", textBody)
	}
}

func TestNormalizeEmailLocale(t *testing.T) {
	cases := map[string]string{
		"is":    "is",
		"is-IS": "is",
		"IS_is": "is",
		"en":    "en",
		"en-US": "en",
		"fr":    "en",
		"":      "en",
		"  is ": "is",
	}
	for input, want := range cases {
		if got := normalizeEmailLocale(input); got != want {
			t.Errorf("normalizeEmailLocale(%q) = %q, want %q", input, got, want)
		}
	}
}
