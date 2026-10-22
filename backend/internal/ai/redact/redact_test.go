package redact

import (
	"strings"
	"testing"
)

func TestValidIIN(t *testing.T) {
	for _, ok := range []string{"950315400125", "020510601231", "881231405672"} {
		if !ValidIIN(ok) {
			t.Errorf("%s rejected", ok)
		}
	}
	for _, bad := range []string{"950315400126", "123456789012", "95031540012", "9503154001250", "95031540012a"} {
		if ValidIIN(bad) {
			t.Errorf("%s accepted", bad)
		}
	}
}

func TestRedactAndRestore(t *testing.T) {
	r := New()
	text := "Пишите на aigerim.s@kazutb.edu.kz или звоните +7 (701) 234-56-78, 8 727 250 11 22. " +
		"ИИН 950315400125, сумма 120000000000 тенге. Второй раз: aigerim.s@kazutb.edu.kz. Tel +44 20 7946 0958."
	red := r.Redact(text)
	for _, leaked := range []string{"aigerim", "701", "250 11 22", "950315400125", "7946"} {
		if strings.Contains(red, leaked) {
			t.Fatalf("%q leaked: %s", leaked, red)
		}
	}
	if !strings.Contains(red, "120000000000") {
		t.Fatalf("a number that is not an IIN must stay: %s", red)
	}
	if strings.Count(red, "[EMAIL_1]") != 2 || !strings.Contains(red, "[PHONE_3]") || !strings.Contains(red, "[IIN_1]") {
		t.Fatalf("placeholders: %s", red)
	}
	if r.Count() != 5 {
		t.Fatalf("count %d", r.Count())
	}
	if got := r.Restore(red); got != text {
		t.Fatalf("restore:\n%s\n%s", got, text)
	}
	if got := r.Restore("Unknown [EMAIL_9] stays"); got != "Unknown [EMAIL_9] stays" {
		t.Fatal(got)
	}
}

func TestStreamRestoresSplitPlaceholders(t *testing.T) {
	r := New()
	r.Redact("mail me at dana@qadam.test")
	s := r.Stream()
	var out strings.Builder
	for _, piece := range []string{"Напишите на [EM", "AIL_", "1] — ", "это быстро [", "скоро]"} {
		out.WriteString(s.Push(piece))
	}
	out.WriteString(s.Flush())
	if out.String() != "Напишите на dana@qadam.test — это быстро [скоро]" {
		t.Fatalf("got %q", out.String())
	}
}
