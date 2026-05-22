package email

import "context"

type Sender struct {
	Mailer      Mailer
	DefaultFrom Address
}

func (s Sender) SendMagicLinkLogin(ctx context.Context, to Address, data MagicLinkTemplateData) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderMagicLinkLogin(data)
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

func (s Sender) SendMagicLinkVerify(ctx context.Context, to Address, data MagicLinkTemplateData) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderMagicLinkVerify(data)
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

func (s Sender) SendBillingReceipt(ctx context.Context, to Address, data BillingReceiptTemplateData, idempotencyKey string) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderBillingReceipt(data)
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

func (s Sender) SendBillingPaymentFailed(ctx context.Context, to Address, data BillingPaymentFailedTemplateData, idempotencyKey string) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderBillingPaymentFailed(data)
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

func (s Sender) SendOnceOverIntakeReady(ctx context.Context, to Address, data OnceOverIntakeReadyTemplateData, idempotencyKey string) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderOnceOverIntakeReady(data)
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

func (s Sender) SendOnceOverDelivered(ctx context.Context, to Address, data OnceOverDeliveredTemplateData) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderOnceOverDelivered(data)
	if err != nil {
		return SendResult{}, err
	}
	return s.Mailer.Send(ctx, Message{
		To:       []Address{to},
		From:     s.DefaultFrom,
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
		Tags:     map[string]string{"template": "once_over_delivered"},
	})
}

func (s Sender) SendFormSubmissionForwarded(ctx context.Context, to Address, data FormSubmissionForwardedTemplateData, idempotencyKey string) (SendResult, error) {
	subject, textBody, htmlBody, err := RenderFormSubmissionForwarded(data)
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
