package clipmind

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/accounts"
)

// Mailer delivers one message. The server queues every email in the catalog
// first, so a mail provider that is down delays the message and loses nothing.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// LogMailer prints messages to the server log. It is for development and
// tests: the bodies carry live links.
type LogMailer struct{}

func (LogMailer) Send(_ context.Context, to, subject, body string) error {
	log.Printf("mail to=%s subject=%q\n%s", to, subject, body)
	return nil
}

// SMTPMailer sends through an SMTP server with STARTTLS (port 587).
type SMTPMailer struct {
	Addr     string
	Username string
	Password string
	From     string
}

func (m SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	if strings.ContainsAny(to+subject+m.From, "\r\n") {
		return fmt.Errorf("invalid header characters")
	}
	host := m.Addr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, host)
	}
	msg := "From: ClipMind <" + m.From + ">\r\nTo: " + to + "\r\nSubject: " + subject +
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n")
	return smtp.SendMail(m.Addr, auth, m.From, []string{to}, []byte(msg))
}

// dispatchEmails sends queued messages until ctx ends. kick wakes it early.
func dispatchEmails(ctx context.Context, svc *accounts.Service, mailer Mailer, kick <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		emails, err := svc.PendingEmails(ctx, 20)
		if err != nil && ctx.Err() == nil {
			log.Printf("email outbox: %v", err)
		}
		for _, e := range emails {
			if err := mailer.Send(ctx, e.To, e.Subject, e.Body); err != nil {
				log.Printf("email %d failed: %v", e.ID, err)
				_ = svc.MarkEmailFailed(ctx, e.ID, err)
				continue
			}
			_ = svc.MarkEmailSent(ctx, e.ID)
		}
		select {
		case <-ctx.Done():
			return
		case <-kick:
		case <-ticker.C:
		}
	}
}
