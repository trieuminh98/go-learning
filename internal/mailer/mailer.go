package mailer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go-learning/internal/logic"
)

var _ logic.Mailer = (*DevMailer)(nil)

// DevMailer provides a local development and test implementation of logic.Mailer.
// It logs outgoing activation emails and supports simulating dispatch failures
// for recipient addresses starting with a configured failure prefix (default: "fail@").
type DevMailer struct {
	failPrefix string
}

// NewDevMailer constructs a DevMailer with default failure prefix "fail@".
func NewDevMailer() *DevMailer {
	return &DevMailer{failPrefix: "fail@"}
}

// SendActivationEmail logs the activation email payload or returns a simulated network failure.
func (m *DevMailer) SendActivationEmail(ctx context.Context, recipientEmail, plainToken string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if strings.HasPrefix(strings.ToLower(recipientEmail), m.failPrefix) {
		return errors.New("simulated mailer failure: SMTP connection timeout")
	}

	fmt.Printf("[DEV MAILER] Activation email dispatched to %s with token: %s\n", recipientEmail, plainToken)
	return nil
}
