package email

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidMessage      = errors.New("email: invalid message")
	ErrRateLimited         = errors.New("email: rate limited")
	ErrProviderUnavailable = errors.New("email: provider unavailable")
	ErrPermanent           = errors.New("email: permanent failure")
)

type Message struct {
	To             []Address
	Cc             []Address
	Bcc            []Address
	From           Address
	ReplyTo        *Address
	Subject        string
	TextBody       string
	HTMLBody       string
	Tags           map[string]string
	IdempotencyKey string
}

type Address struct {
	Email string
	Name  string
}

type SendResult struct {
	ProviderMessageID string
	AcceptedAt        time.Time
}

type Mailer interface {
	Send(ctx context.Context, msg Message) (SendResult, error)
}

func ValidateMessage(msg Message) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("%w: at least one recipient is required", ErrInvalidMessage)
	}
	for _, address := range append(append([]Address{}, msg.To...), append(msg.Cc, msg.Bcc...)...) {
		if address.Email == "" {
			return fmt.Errorf("%w: recipient email is required", ErrInvalidMessage)
		}
	}
	if msg.Subject == "" {
		return fmt.Errorf("%w: subject is required", ErrInvalidMessage)
	}
	if msg.TextBody == "" && msg.HTMLBody == "" {
		return fmt.Errorf("%w: text or html body is required", ErrInvalidMessage)
	}
	return nil
}
