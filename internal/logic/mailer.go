package logic

import (
	"context"
)

// Mailer abstracts sending outgoing transactional emails (such as user activation links).
// Kept decoupled from SMTP or specific email vendor implementations.
type Mailer interface {
	SendActivationEmail(ctx context.Context, recipientEmail, plainToken string) error
}
