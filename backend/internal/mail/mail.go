// Package mail renders and delivers e-mail in Kazakh, Russian and English.
// Services enqueue letters as River jobs; the worker renders and sends them.
package mail

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"time"

	"github.com/riverqueue/river"
	gomail "github.com/wneessen/go-mail"

	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/jobs"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
)

//go:embed layout.html
var layoutHTML string

var layout = template.Must(template.New("layout").Parse(layoutHTML))

var footers = map[string]string{
	"kk": "Бұл хат Qadam платформасынан автоматты түрде жіберілді — университеттің инновациялық және жобалық қызметі. Жауап бермеңіз.",
	"ru": "Письмо отправлено автоматически платформой Qadam — инновационной и проектной деятельности университета. Не отвечайте на него.",
	"en": "This letter was sent automatically by Qadam, the university innovation and project platform. Please do not reply.",
}

// Message is a rendered letter.
type Message struct {
	To      string
	ToName  string
	Subject string
	HTML    string
	Text    string
}

// Render produces the letter in the locale (falling back to Russian), with
// placeholders filled from data. Data values are escaped by html/template.
func Render(tpl Template, locale string, data map[string]string) (Message, error) {
	letters, ok := catalog[tpl]
	if !ok {
		return Message{}, fmt.Errorf("mail: unknown template %q", tpl)
	}
	c, ok := letters[locale]
	if !ok {
		locale = "ru"
		c = letters[locale]
	}
	pairs := make([]string, 0, len(data)*2)
	for k, v := range data {
		pairs = append(pairs, "{"+k+"}", v)
	}
	fill := strings.NewReplacer(pairs...).Replace

	paragraphs := make([]string, len(c.Paragraphs))
	for i, p := range c.Paragraphs {
		paragraphs[i] = fill(p)
	}
	subject, heading, code, after := fill(c.Subject), fill(c.Heading), fill(c.Code), fill(c.After)
	view := map[string]any{
		"Lang":       locale,
		"Subject":    subject,
		"Preheader":  fill(c.Preheader),
		"Heading":    heading,
		"Paragraphs": paragraphs,
		"Code":       code,
		"Button":     c.Button,
		"Link":       data["link"],
		"After":      after,
		"Footer":     footers[locale],
	}
	var html bytes.Buffer
	if err := layout.Execute(&html, view); err != nil {
		return Message{}, fmt.Errorf("mail: render %s: %w", tpl, err)
	}

	var text strings.Builder
	text.WriteString(heading + "\n\n")
	for _, p := range paragraphs {
		text.WriteString(p + "\n\n")
	}
	if code != "" {
		text.WriteString(code + "\n\n")
	}
	if c.Button != "" {
		text.WriteString(c.Button + ": " + data["link"] + "\n\n")
	}
	if after != "" {
		text.WriteString(after + "\n\n")
	}
	text.WriteString("— Qadam\n")

	return Message{Subject: subject, HTML: html.String(), Text: text.String()}, nil
}

// Sender delivers rendered letters.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// SMTPSender delivers through an SMTP relay.
type SMTPSender struct {
	cfg config.SMTP
}

// NewSender returns an SMTP sender, or a log-only sender when no host is set.
func NewSender(cfg config.SMTP, log *slog.Logger) Sender {
	if cfg.Host == "" {
		return logSender{log: log}
	}
	return &SMTPSender{cfg: cfg}
}

// Send implements Sender.
func (s *SMTPSender) Send(ctx context.Context, m Message) error {
	msg := gomail.NewMsg()
	if err := msg.FromFormat(s.cfg.FromName, s.cfg.From); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := msg.AddToFormat(m.ToName, m.To); err != nil {
		return fmt.Errorf("mail: to: %w", err)
	}
	msg.Subject(m.Subject)
	msg.SetBodyString(gomail.TypeTextPlain, m.Text)
	msg.AddAlternativeString(gomail.TypeTextHTML, m.HTML)

	opts := []gomail.Option{gomail.WithPort(s.cfg.Port), gomail.WithTimeout(20 * time.Second)}
	switch s.cfg.TLS {
	case "tls":
		opts = append(opts, gomail.WithSSL())
	case "starttls":
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	default:
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	}
	if s.cfg.Username != "" {
		opts = append(opts, gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(s.cfg.Username), gomail.WithPassword(s.cfg.Password))
	}
	client, err := gomail.NewClient(s.cfg.Host, opts...)
	if err != nil {
		return fmt.Errorf("mail: client: %w", err)
	}
	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("mail: send: %w", err)
	}
	return nil
}

type logSender struct{ log *slog.Logger }

func (l logSender) Send(ctx context.Context, m Message) error {
	l.log.WarnContext(ctx, "SMTP is not configured; letter not delivered", slog.String("subject", m.Subject))
	return nil
}

// payload is the sealed part of a job: who receives the letter and its data
// (sign-in codes, reset links) never sit readable in the jobs table.
type payload struct {
	To     string            `json:"to"`
	ToName string            `json:"to_name"`
	Data   map[string]string `json:"data"`
}

// SendArgs is the River job delivering one letter.
type SendArgs struct {
	Template Template `json:"template"`
	Locale   string   `json:"locale"`
	Sealed   []byte   `json:"sealed"`
}

// Kind implements river.JobArgs.
func (SendArgs) Kind() string { return "mail.send" }

// InsertOpts routes letters to their own queue with generous retries.
func (SendArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueName, MaxAttempts: 10}
}

// QueueName is the River queue of letters.
const QueueName = "mail"

// Mailer enqueues letters.
type Mailer struct {
	queue jobs.Enqueuer
	keys  *secrets.Keyring
}

// NewMailer returns a mailer that enqueues sealed letters.
func NewMailer(queue jobs.Enqueuer, keys *secrets.Keyring) *Mailer {
	return &Mailer{queue: queue, keys: keys}
}

// Send enqueues a letter (inside the transaction of ctx, if any).
func (m *Mailer) Send(ctx context.Context, to, toName string, tpl Template, locale string, data map[string]string) error {
	raw, err := json.Marshal(payload{To: to, ToName: toName, Data: data})
	if err != nil {
		return fmt.Errorf("mail: encode: %w", err)
	}
	sealed, err := m.keys.Seal(raw)
	if err != nil {
		return err
	}
	return m.queue.Enqueue(ctx, SendArgs{Template: tpl, Locale: locale, Sealed: sealed}, nil)
}

// SendWorker renders and delivers letters.
type SendWorker struct {
	river.WorkerDefaults[SendArgs]
	Sender Sender
	Keys   *secrets.Keyring
}

// Work implements river.Worker.
func (w *SendWorker) Work(ctx context.Context, job *river.Job[SendArgs]) error {
	raw, err := w.Keys.Open(job.Args.Sealed)
	if err != nil {
		return river.JobCancel(fmt.Errorf("mail: unseal: %w", err))
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return river.JobCancel(fmt.Errorf("mail: decode: %w", err))
	}
	msg, err := Render(job.Args.Template, job.Args.Locale, p.Data)
	if err != nil {
		return river.JobCancel(err)
	}
	msg.To, msg.ToName = p.To, p.ToName
	return w.Sender.Send(ctx, msg)
}

// Timeout bounds one delivery attempt.
func (w *SendWorker) Timeout(*river.Job[SendArgs]) time.Duration { return 45 * time.Second }
