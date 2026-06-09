package email

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"
)

//go:embed templates/*.txt templates/*.html
var templateFS embed.FS

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

func RenderMagicLinkLogin(data MagicLinkTemplateData) (string, string, string, error) {
	return renderTemplate("magic_link_login", "Your Snaelda login link", data)
}

func RenderMagicLinkVerify(data MagicLinkTemplateData) (string, string, string, error) {
	return renderTemplate("magic_link_verify", "Verify your Snaelda email", data)
}

func RenderBillingReceipt(data BillingReceiptTemplateData) (string, string, string, error) {
	subject := fmt.Sprintf("Your %s receipt", data.ProductName)
	return renderTemplate("billing_receipt", subject, data)
}

func RenderBillingPaymentFailed(data BillingPaymentFailedTemplateData) (string, string, string, error) {
	subject := fmt.Sprintf("%s payment failed", data.ProductName)
	return renderTemplate("billing_payment_failed", subject, data)
}

func RenderOnceOverIntakeReady(data OnceOverIntakeReadyTemplateData) (string, string, string, error) {
	subject := fmt.Sprintf("%s once-over intake is ready", data.ProductName)
	return renderTemplate("once_over_intake_ready", subject, data)
}

func RenderOnceOverDelivered(data OnceOverDeliveredTemplateData) (string, string, string, error) {
	subject := fmt.Sprintf("%s once-over is ready", data.ProductName)
	return renderTemplate("once_over_delivered", subject, data)
}

func RenderFormSubmissionForwarded(data FormSubmissionForwardedTemplateData) (string, string, string, error) {
	subject := fmt.Sprintf("New form submission for %s", data.SiteName)
	return renderTemplate("form_submission_forwarded", subject, data)
}

func renderTemplate(name string, subject string, data any) (string, string, string, error) {
	textBody, err := executeTextTemplate(name+".txt", data)
	if err != nil {
		return "", "", "", err
	}
	htmlBody, err := executeHTMLTemplate(name+".html", data)
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
