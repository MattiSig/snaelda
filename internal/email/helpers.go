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
