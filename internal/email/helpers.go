package email

import "context"

type Sender struct {
	Mailer      Mailer
	DefaultFrom Address
}

func (s Sender) SendMagicLinkLogin(ctx context.Context, to Address, locale string, data MagicLinkTemplateData) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderMagicLinkLogin(locale, data)
	if err != nil {
		return SendResult{}, err
	}
	return s.Mailer.Send(ctx, Message{
		To:       []Address{to},
		From:     s.DefaultFrom,
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
		Tags:     map[string]string{"template": "magic_link_login"},
	})
}

func (s Sender) SendMagicLinkVerify(ctx context.Context, to Address, locale string, data MagicLinkTemplateData) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderMagicLinkVerify(locale, data)
	if err != nil {
		return SendResult{}, err
	}
	return s.Mailer.Send(ctx, Message{
		To:       []Address{to},
		From:     s.DefaultFrom,
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
		Tags:     map[string]string{"template": "magic_link_verify"},
	})
}

func (s Sender) SendBillingReceipt(ctx context.Context, to Address, locale string, data BillingReceiptTemplateData, idempotencyKey string) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderBillingReceipt(locale, data)
	if err != nil {
		return SendResult{}, err
	}
	return s.Mailer.Send(ctx, Message{
		To:             []Address{to},
		From:           s.DefaultFrom,
		Subject:        subject,
		TextBody:       textBody,
		HTMLBody:       htmlBody,
		Tags:           map[string]string{"template": "billing_receipt"},
		IdempotencyKey: idempotencyKey,
	})
}

func (s Sender) SendBillingPaymentFailed(ctx context.Context, to Address, locale string, data BillingPaymentFailedTemplateData, idempotencyKey string) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderBillingPaymentFailed(locale, data)
	if err != nil {
		return SendResult{}, err
	}
	return s.Mailer.Send(ctx, Message{
		To:             []Address{to},
		From:           s.DefaultFrom,
		Subject:        subject,
		TextBody:       textBody,
		HTMLBody:       htmlBody,
		Tags:           map[string]string{"template": "billing_payment_failed"},
		IdempotencyKey: idempotencyKey,
	})
}

func (s Sender) SendOnceOverIntakeReady(ctx context.Context, to Address, locale string, data OnceOverIntakeReadyTemplateData, idempotencyKey string) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderOnceOverIntakeReady(locale, data)
	if err != nil {
		return SendResult{}, err
	}
	return s.Mailer.Send(ctx, Message{
		To:             []Address{to},
		From:           s.DefaultFrom,
		Subject:        subject,
		TextBody:       textBody,
		HTMLBody:       htmlBody,
		Tags:           map[string]string{"template": "once_over_intake_ready"},
		IdempotencyKey: idempotencyKey,
	})
}

func (s Sender) SendOnceOverDelivered(ctx context.Context, to Address, locale string, data OnceOverDeliveredTemplateData, idempotencyKey string) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderOnceOverDelivered(locale, data)
	if err != nil {
		return SendResult{}, err
	}
	return s.Mailer.Send(ctx, Message{
		To:             []Address{to},
		From:           s.DefaultFrom,
		Subject:        subject,
		TextBody:       textBody,
		HTMLBody:       htmlBody,
		Tags:           map[string]string{"template": "once_over_delivered"},
		IdempotencyKey: idempotencyKey,
	})
}

func (s Sender) SendWorkspaceClaimed(ctx context.Context, to Address, locale string, data WorkspaceClaimedTemplateData, idempotencyKey string) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderWorkspaceClaimed(locale, data)
	if err != nil {
		return SendResult{}, err
	}
	return s.Mailer.Send(ctx, Message{
		To:             []Address{to},
		From:           s.DefaultFrom,
		Subject:        subject,
		TextBody:       textBody,
		HTMLBody:       htmlBody,
		Tags:           map[string]string{"template": "workspace_claimed"},
		IdempotencyKey: idempotencyKey,
	})
}

func (s Sender) SendFormSubmissionForwarded(ctx context.Context, to Address, locale string, data FormSubmissionForwardedTemplateData, idempotencyKey string) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderFormSubmissionForwarded(locale, data)
	if err != nil {
		return SendResult{}, err
	}
	return s.Mailer.Send(ctx, Message{
		To:             []Address{to},
		From:           s.DefaultFrom,
		Subject:        subject,
		TextBody:       textBody,
		HTMLBody:       htmlBody,
		Tags:           map[string]string{"template": "form_submission_forwarded"},
		IdempotencyKey: idempotencyKey,
	})
}
