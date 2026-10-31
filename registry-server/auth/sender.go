package auth

import (
	"context"
	"log"
)

// LogSender "sends" verification emails by logging them to the server's
// own log output. This is Milestone 3's only EmailSender — there is no
// real mail transport yet (ROADMAP.md tracks this as a known gap, not a
// silent assumption). It exists so the registration/verification flow
// is fully testable and runnable locally without external services.
type LogSender struct{}

func (LogSender) Send(_ context.Context, to, verifyURL string) error {
	log.Printf("[auth] verification email for %s: %s", to, verifyURL)
	return nil
}
