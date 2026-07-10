package email

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
)

//go:embed templates/*/*.txt templates/*/*.html
var templateFS embed.FS

// SupportedLocales is the set of locales every transactional template ships in.
// Icelandic leads (Spec 22); English is the guaranteed fallback (Spec 18).
var SupportedLocales = []string{"is", "en"}

const defaultEmailLocale = "en"

// normalizeEmailLocale coerces an arbitrary locale hint (a workspace locale, a
// site default locale) to the supported email set, reducing to the primary
// subtag ("is-IS" -> "is") and falling back to English per Spec 18.
func normalizeEmailLocale(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if idx := strings.IndexAny(v, "-_"); idx > 0 {
		v = v[:idx]
	}
	if v == "is" {
		return "is"
	}
	return defaultEmailLocale
}

// subjectFormats holds every template's subject line keyed by template name and
// then locale. Values are fmt format strings; templates whose subject embeds a
// runtime value (product or site name) carry a single %s verb.
var subjectFormats = map[string]map[string]string{
	"magic_link_login": {
		"en": "Your %s login link",
		"is": "Innskráningarhlekkurinn þinn hjá %s",
	},
	"magic_link_verify": {
		"en": "Verify your %s email",
		"is": "Staðfestu netfangið þitt hjá %s",
	},
	"billing_receipt": {
		"en": "Your %s receipt",
		"is": "Kvittunin þín frá %s",
	},
	"billing_payment_failed": {
		"en": "%s payment failed",
		"is": "Greiðslan hjá %s tókst ekki",
	},
	"once_over_intake_ready": {
		"en": "%s once-over intake is ready",
		"is": "Yfirferðin þín hjá %s er tilbúin til að hefjast",
	},
	"once_over_delivered": {
		"en": "%s once-over is ready",
		"is": "Yfirferðin þín hjá %s er tilbúin",
	},
	"form_submission_forwarded": {
		"en": "New form submission for %s",
		"is": "Ný fyrirspurn í gegnum %s",
	},
	"workspace_claimed": {
		"en": "Your %s workspace is saved",
		"is": "Vinnusvæðið þitt hjá %s er vistað",
	},
}

// subjectFor renders a template's localized subject, interpolating the supplied
// arguments into the locale format string. It falls back to the English subject
// when a locale variant is missing.
func subjectFor(name, locale string, args ...any) string {
	byLocale, ok := subjectFormats[name]
	if !ok {
		return ""
	}
	format, ok := byLocale[locale]
	if !ok {
		format = byLocale[defaultEmailLocale]
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

type MagicLinkTemplateData struct {
	ProductName string
	ActionLabel string
	Email       string
	MagicURL    string
	ExpiresIn   string
}

type BillingReceiptTemplateData struct {
	ProductName   string
	WorkspaceName string
	Amount        string
	Currency      string
	ReceiptURL    string
	PlanName      string
}

type BillingPaymentFailedTemplateData struct {
	ProductName   string
	WorkspaceName string
	PlanName      string
	PortalURL     string
}

type OnceOverIntakeReadyTemplateData struct {
	ProductName   string
	WorkspaceName string
	IntakeURL     string
}

type OnceOverDeliveredTemplateData struct {
	ProductName   string
	WorkspaceName string
	DeliveryURL   string
	NextSteps     []string
}

type WorkspaceClaimedTemplateData struct {
	ProductName   string
	WorkspaceName string
	LoginURL      string
}

type FormSubmissionForwardedTemplateData struct {
	ProductName string
	SiteName    string
	PageTitle   string
	SubmittedAt string
	Fields      []ForwardedField
}

type ForwardedField struct {
	Label string
	Value string
}

func RenderMagicLinkLogin(locale string, data MagicLinkTemplateData) (string, string, string, error) {
	locale = normalizeEmailLocale(locale)
	return renderTemplate("magic_link_login", subjectFor("magic_link_login", locale, data.ProductName), locale, data)
}

func RenderMagicLinkVerify(locale string, data MagicLinkTemplateData) (string, string, string, error) {
	locale = normalizeEmailLocale(locale)
	return renderTemplate("magic_link_verify", subjectFor("magic_link_verify", locale, data.ProductName), locale, data)
}

func RenderBillingReceipt(locale string, data BillingReceiptTemplateData) (string, string, string, error) {
	locale = normalizeEmailLocale(locale)
	return renderTemplate("billing_receipt", subjectFor("billing_receipt", locale, data.ProductName), locale, data)
}

func RenderBillingPaymentFailed(locale string, data BillingPaymentFailedTemplateData) (string, string, string, error) {
	locale = normalizeEmailLocale(locale)
	return renderTemplate("billing_payment_failed", subjectFor("billing_payment_failed", locale, data.ProductName), locale, data)
}

func RenderOnceOverIntakeReady(locale string, data OnceOverIntakeReadyTemplateData) (string, string, string, error) {
	locale = normalizeEmailLocale(locale)
	return renderTemplate("once_over_intake_ready", subjectFor("once_over_intake_ready", locale, data.ProductName), locale, data)
}

func RenderOnceOverDelivered(locale string, data OnceOverDeliveredTemplateData) (string, string, string, error) {
	locale = normalizeEmailLocale(locale)
	return renderTemplate("once_over_delivered", subjectFor("once_over_delivered", locale, data.ProductName), locale, data)
}

func RenderFormSubmissionForwarded(locale string, data FormSubmissionForwardedTemplateData) (string, string, string, error) {
	locale = normalizeEmailLocale(locale)
	return renderTemplate("form_submission_forwarded", subjectFor("form_submission_forwarded", locale, data.SiteName), locale, data)
}

func RenderWorkspaceClaimed(locale string, data WorkspaceClaimedTemplateData) (string, string, string, error) {
	locale = normalizeEmailLocale(locale)
	return renderTemplate("workspace_claimed", subjectFor("workspace_claimed", locale, data.ProductName), locale, data)
}

func renderTemplate(name string, subject string, locale string, data any) (string, string, string, error) {
	textBody, err := executeTextTemplate(locale+"/"+name+".txt", data)
	if err != nil {
		return "", "", "", err
	}
	htmlBody, err := executeHTMLTemplate(locale+"/"+name+".html", data)
	if err != nil {
		return "", "", "", err
	}
	return subject, textBody, htmlBody, nil
}

func executeTextTemplate(name string, data any) (string, error) {
	tpl, err := texttemplate.ParseFS(templateFS, "templates/"+name)
	if err != nil {
		return "", fmt.Errorf("parse text template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render text template %s: %w", name, err)
	}
	return buf.String(), nil
}

func executeHTMLTemplate(name string, data any) (string, error) {
	tpl, err := htmltemplate.ParseFS(templateFS, "templates/"+name)
	if err != nil {
		return "", fmt.Errorf("parse html template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render html template %s: %w", name, err)
	}
	return buf.String(), nil
}
