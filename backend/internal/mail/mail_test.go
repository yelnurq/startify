package mail

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/riverqueue/river"

	"github.com/yelnurq/startify/backend/internal/platform/secrets"
)

func TestEveryLetterRendersInEveryLanguage(t *testing.T) {
	data := map[string]string{
		"name": "Айгерім", "tenant": "КазУТБ", "link": "https://kazutb.qadam.kz/x?token=abc",
		"code": "482913", "minutes": "10", "responder": "Нұрлан", "request": "Ищем дизайнера", "message": "Готов помочь с UX",
		"project": "AgroScan", "sender": "Ержан (AgroInvest)", "kind": "инвестиции", "email": "erzhan@agroinvest.kz",
	}
	for tpl, letters := range catalog {
		for _, locale := range []string{"kk", "ru", "en"} {
			if _, ok := letters[locale]; !ok {
				t.Fatalf("%s has no %s version", tpl, locale)
			}
			msg, err := Render(tpl, locale, data)
			if err != nil {
				t.Fatalf("%s/%s: %v", tpl, locale, err)
			}
			if strings.Contains(msg.Subject+msg.HTML+msg.Text, "{") {
				t.Errorf("%s/%s: unfilled placeholder in %q", tpl, locale, msg.Subject)
			}
			if !strings.Contains(msg.HTML, `lang="`+locale+`"`) {
				t.Errorf("%s/%s: html lang missing", tpl, locale)
			}
		}
	}
}

func TestRenderEscapesData(t *testing.T) {
	msg, err := Render(TemplateTeamResponse, "ru", map[string]string{
		"responder": "<script>alert(1)</script>", "request": "R", "message": "M", "link": "https://x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg.HTML, "<script>") {
		t.Fatal("data must be HTML-escaped")
	}
}

func TestRenderFallsBackToRussian(t *testing.T) {
	msg, err := Render(TemplateLoginCode, "de", map[string]string{"code": "111222", "minutes": "10", "tenant": "T"})
	if err != nil || !strings.Contains(msg.Subject, "Код для входа") {
		t.Fatalf("got %q, %v", msg.Subject, err)
	}
}

type recordingSender struct{ sent []Message }

func (r *recordingSender) Send(_ context.Context, m Message) error {
	r.sent = append(r.sent, m)
	return nil
}

type queued struct{ args []river.JobArgs }

func (q *queued) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	q.args = append(q.args, args)
	return nil
}

func TestMailerSealsAndWorkerDelivers(t *testing.T) {
	keys, err := secrets.NewKeyring(strings.Repeat("s", 40))
	if err != nil {
		t.Fatal(err)
	}
	q := &queued{}
	err = NewMailer(q, keys).Send(context.Background(), "aigerim@qadam.test", "Айгерім", TemplateLoginCode, "kk",
		map[string]string{"code": "482913", "minutes": "10", "tenant": "ҚазТБУ", "name": "Айгерім"})
	if err != nil {
		t.Fatal(err)
	}
	args := q.args[0].(SendArgs)
	if strings.Contains(string(args.Sealed), "482913") || strings.Contains(string(args.Sealed), "aigerim") {
		t.Fatal("the code and the address must not be stored readable")
	}
	if args.InsertOpts().Queue != QueueName {
		t.Fatal("letters go to their own queue")
	}

	sender := &recordingSender{}
	w := &SendWorker{Sender: sender, Keys: keys}
	if err := w.Work(context.Background(), &river.Job[SendArgs]{Args: args}); err != nil {
		t.Fatal(err)
	}
	m := sender.sent[0]
	if m.To != "aigerim@qadam.test" || !strings.Contains(m.HTML, "482913") || !strings.Contains(m.Subject, "Qadam") {
		t.Fatalf("delivered %+v", m)
	}

	other, _ := secrets.NewKeyring(strings.Repeat("o", 40))
	err = (&SendWorker{Sender: sender, Keys: other}).Work(context.Background(), &river.Job[SendArgs]{Args: args})
	var cancel *river.JobCancelError
	if !errors.As(err, &cancel) {
		t.Fatalf("a letter sealed with another key must be cancelled, got %v", err)
	}
}
