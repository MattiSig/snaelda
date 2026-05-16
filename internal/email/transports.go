package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Transport       string
	DefaultFrom     Address
	ReplyTo         *Address
	ResendAPIKey    string
	MailpitSMTPAddr string
	Logger          *slog.Logger
	HTTPClient      *http.Client
	Now             func() time.Time
}

func NewMailer(cfg Config) (Mailer, error) {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	defaultFrom := cfg.DefaultFrom
	if defaultFrom.Email == "" {
		return nil, fmt.Errorf("email default from address is required")
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Transport)) {
	case "", "stdout":
		return &stdoutMailer{defaultFrom: defaultFrom, replyTo: cfg.ReplyTo, logger: logger, now: now}, nil
	case "mailpit":
		addr := strings.TrimSpace(cfg.MailpitSMTPAddr)
		if addr == "" {
			addr = "localhost:1025"
		}
		return &smtpMailer{defaultFrom: defaultFrom, replyTo: cfg.ReplyTo, addr: addr, now: now}, nil
	case "resend":
		if strings.TrimSpace(cfg.ResendAPIKey) == "" {
			return nil, fmt.Errorf("RESEND_API_KEY is required when EMAIL_TRANSPORT=resend")
		}
		client := cfg.HTTPClient
		if client == nil {
			client = &http.Client{Timeout: 15 * time.Second}
		}
		return &resendMailer{
			defaultFrom: defaultFrom,
			replyTo:     cfg.ReplyTo,
			apiKey:      strings.TrimSpace(cfg.ResendAPIKey),
			httpClient:  client,
			now:         now,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported EMAIL_TRANSPORT %q", cfg.Transport)
	}
}

type stdoutMailer struct {
	defaultFrom Address
	replyTo     *Address
	logger      *slog.Logger
	now         func() time.Time
}

func (m *stdoutMailer) Send(_ context.Context, msg Message) (SendResult, error) {
	msg = applyDefaults(msg, m.defaultFrom, m.replyTo)
	if err := ValidateMessage(msg); err != nil {
		return SendResult{}, err
	}
	acceptedAt := m.now().UTC()
	payload, _ := json.Marshal(map[string]any{
		"to":         msg.To,
		"cc":         msg.Cc,
		"bcc":        msg.Bcc,
		"from":       msg.From,
		"replyTo":    msg.ReplyTo,
		"subject":    msg.Subject,
		"textBody":   msg.TextBody,
		"htmlBody":   msg.HTMLBody,
		"tags":       msg.Tags,
		"acceptedAt": acceptedAt.Format(time.RFC3339),
	})
	m.logger.Info("email send", "transport", "stdout", "message", string(payload))
	return SendResult{
		ProviderMessageID: fmt.Sprintf("stdout-%d", acceptedAt.UnixNano()),
		AcceptedAt:        acceptedAt,
	}, nil
}

type smtpMailer struct {
	defaultFrom Address
	replyTo     *Address
	addr        string
	now         func() time.Time
}

func (m *smtpMailer) Send(_ context.Context, msg Message) (SendResult, error) {
	msg = applyDefaults(msg, m.defaultFrom, m.replyTo)
	if err := ValidateMessage(msg); err != nil {
		return SendResult{}, err
	}

	var raw bytes.Buffer
	writeMIMEMessage(&raw, msg)
	if err := smtp.SendMail(m.addr, nil, msg.From.Email, recipientEmails(msg), raw.Bytes()); err != nil {
		return SendResult{}, classifySendError(err)
	}

	return SendResult{
		ProviderMessageID: fmt.Sprintf("smtp-%d", m.now().UTC().UnixNano()),
		AcceptedAt:        m.now().UTC(),
	}, nil
}

type resendMailer struct {
	defaultFrom Address
	replyTo     *Address
	apiKey      string
	httpClient  *http.Client
	now         func() time.Time
}

func (m *resendMailer) Send(ctx context.Context, msg Message) (SendResult, error) {
	msg = applyDefaults(msg, m.defaultFrom, m.replyTo)
	if err := ValidateMessage(msg); err != nil {
		return SendResult{}, err
	}

	payload := map[string]any{
		"from":    formatAddress(msg.From),
		"to":      addressStrings(msg.To),
		"cc":      addressStrings(msg.Cc),
		"bcc":     addressStrings(msg.Bcc),
		"subject": msg.Subject,
		"text":    msg.TextBody,
		"html":    msg.HTMLBody,
	}
	if msg.ReplyTo != nil && msg.ReplyTo.Email != "" {
		payload["reply_to"] = []string{formatAddress(*msg.ReplyTo)}
	}
	if len(msg.Tags) > 0 {
		payload["tags"] = tagsToResend(msg.Tags)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return SendResult{}, fmt.Errorf("%w: marshal resend payload: %v", ErrProviderUnavailable, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return SendResult{}, fmt.Errorf("%w: build resend request: %v", ErrProviderUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if msg.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", msg.IdempotencyKey)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return SendResult{}, fmt.Errorf("%w: resend request failed: %v", ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return SendResult{}, ErrRateLimited
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return SendResult{}, ErrProviderUnavailable
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return SendResult{}, ErrPermanent
	}

	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return SendResult{}, fmt.Errorf("%w: decode resend response: %v", ErrProviderUnavailable, err)
	}

	return SendResult{
		ProviderMessageID: parsed.ID,
		AcceptedAt:        m.now().UTC(),
	}, nil
}

type MemoryMailer struct {
	mu       sync.Mutex
	now      func() time.Time
	Messages []Message
}

func NewMemoryMailer() *MemoryMailer {
	return &MemoryMailer{now: time.Now}
}

func (m *MemoryMailer) Send(_ context.Context, msg Message) (SendResult, error) {
	if err := ValidateMessage(msg); err != nil {
		return SendResult{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = append(m.Messages, msg)
	acceptedAt := m.now().UTC()
	return SendResult{
		ProviderMessageID: fmt.Sprintf("memory-%d", acceptedAt.UnixNano()),
		AcceptedAt:        acceptedAt,
	}, nil
}

func writeMIMEMessage(buf *bytes.Buffer, msg Message) {
	boundary := fmt.Sprintf("snaelda-%d", time.Now().UnixNano())
	headers := map[string]string{
		"From":         formatAddress(msg.From),
		"To":           strings.Join(addressStrings(msg.To), ", "),
		"Subject":      mime.QEncoding.Encode("utf-8", msg.Subject),
		"MIME-Version": "1.0",
		"Content-Type": fmt.Sprintf("multipart/alternative; boundary=%q", boundary),
	}
	if len(msg.Cc) > 0 {
		headers["Cc"] = strings.Join(addressStrings(msg.Cc), ", ")
	}
	if msg.ReplyTo != nil && msg.ReplyTo.Email != "" {
		headers["Reply-To"] = formatAddress(*msg.ReplyTo)
	}
	for name, value := range headers {
		buf.WriteString(name + ": " + value + "\r\n")
	}
	buf.WriteString("\r\n")
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	buf.WriteString(msg.TextBody + "\r\n")
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	buf.WriteString(msg.HTMLBody + "\r\n")
	buf.WriteString("--" + boundary + "--\r\n")
}

func applyDefaults(msg Message, defaultFrom Address, defaultReplyTo *Address) Message {
	if msg.From.Email == "" {
		msg.From = defaultFrom
	}
	if msg.ReplyTo == nil && defaultReplyTo != nil {
		replyTo := *defaultReplyTo
		msg.ReplyTo = &replyTo
	}
	return msg
}

func addressStrings(addresses []Address) []string {
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		out = append(out, formatAddress(address))
	}
	return out
}

func recipientEmails(msg Message) []string {
	out := make([]string, 0, len(msg.To)+len(msg.Cc)+len(msg.Bcc))
	for _, address := range append(append([]Address{}, msg.To...), append(msg.Cc, msg.Bcc...)...) {
		out = append(out, address.Email)
	}
	return out
}

func formatAddress(address Address) string {
	if address.Name == "" {
		return address.Email
	}
	return mime.QEncoding.Encode("utf-8", address.Name) + " <" + address.Email + ">"
}

func tagsToResend(tags map[string]string) []map[string]string {
	out := make([]map[string]string, 0, len(tags))
	for key, value := range tags {
		out = append(out, map[string]string{"name": key, "value": value})
	}
	return out
}

func classifySendError(err error) error {
	if err == nil {
		return nil
	}
	var tlsErr tls.RecordHeaderError
	if errors.As(err, &tlsErr) {
		return fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	return fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
}
