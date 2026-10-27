// Package redact hides personal data — Kazakhstan IINs, phone numbers and
// e-mail addresses — before text leaves for a model outside the university,
// and puts the originals back into the answer.
package redact

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Notice tells the model how to treat placeholders.
const Notice = "Personal data in this conversation (e-mail addresses, phone numbers, IINs) was replaced " +
	"with placeholders such as [EMAIL_1], [PHONE_1] and [IIN_1]. Keep placeholders exactly as written " +
	"when you refer to them; never guess what they stand for."

var (
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)*\.[A-Za-z]{2,}`)
	// Kazakhstan numbers: +7 / 8, then 7xx (mobile) or a city code, in any grouping.
	kzPhoneRe = regexp.MustCompile(`(?:\+7|\b8)[\s\-.(]*[67]\d{2}[\s\-.)]*\d{3}[\s\-.]*\d{2}[\s\-.]*\d{2}\b`)
	// Other international numbers need an explicit +country code.
	intlPhoneRe   = regexp.MustCompile(`\+\d{1,3}(?:[\s\-.()]*\d){8,12}\b`)
	digitsRe      = regexp.MustCompile(`\d+`)
	placeholderRe = regexp.MustCompile(`\[(?:EMAIL|PHONE|IIN)_\d+\]`)
)

// Redactor replaces personal data consistently: the same value always gets
// the same placeholder within one request.
type Redactor struct {
	mu       sync.Mutex
	byValue  map[string]string
	byHolder map[string]string
	counts   map[string]int
}

// New returns an empty redactor.
func New() *Redactor {
	return &Redactor{byValue: map[string]string{}, byHolder: map[string]string{}, counts: map[string]int{}}
}

// Count is the number of distinct values hidden so far.
func (r *Redactor) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.byValue)
}

func (r *Redactor) holder(kind, value string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.byValue[value]; ok {
		return h
	}
	r.counts[kind]++
	h := fmt.Sprintf("[%s_%d]", kind, r.counts[kind])
	r.byValue[value] = h
	r.byHolder[h] = value
	return h
}

// Redact hides personal data in text.
func (r *Redactor) Redact(text string) string {
	text = emailRe.ReplaceAllStringFunc(text, func(m string) string { return r.holder("EMAIL", m) })
	text = digitsRe.ReplaceAllStringFunc(text, func(m string) string {
		if len(m) == 12 && ValidIIN(m) {
			return r.holder("IIN", m)
		}
		return m
	})
	text = kzPhoneRe.ReplaceAllStringFunc(text, func(m string) string { return r.holder("PHONE", m) })
	return intlPhoneRe.ReplaceAllStringFunc(text, func(m string) string { return r.holder("PHONE", m) })
}

// Restore puts the originals back.
func (r *Redactor) Restore(text string) string {
	return placeholderRe.ReplaceAllStringFunc(text, func(h string) string {
		r.mu.Lock()
		defer r.mu.Unlock()
		if v, ok := r.byHolder[h]; ok {
			return v
		}
		return h
	})
}

// ValidIIN checks the control digit of a Kazakhstan individual (or business)
// identification number.
func ValidIIN(s string) bool {
	if len(s) != 12 {
		return false
	}
	d := make([]int, 12)
	for i, c := range s {
		if c < '0' || c > '9' {
			return false
		}
		d[i] = int(c - '0')
	}
	sum := 0
	for i := range 11 {
		sum += d[i] * (i + 1)
	}
	check := sum % 11
	if check == 10 {
		weights := []int{3, 4, 5, 6, 7, 8, 9, 10, 11, 1, 2}
		sum = 0
		for i := range 11 {
			sum += d[i] * weights[i]
		}
		check = sum % 11
		if check == 10 {
			return false
		}
	}
	return check == d[11]
}

// maxHolder bounds how long a placeholder can be ("[EMAIL_999]").
const maxHolder = 12

// Stream restores placeholders in a streamed answer whose pieces may split a
// placeholder: text that could still become one is held back.
type Stream struct {
	r       *Redactor
	pending string
}

// Stream starts restoring a streamed answer.
func (r *Redactor) Stream() *Stream { return &Stream{r: r} }

// Push takes the next piece and returns the text that is safe to show.
func (s *Stream) Push(delta string) string {
	s.pending += delta
	cut := len(s.pending)
	if i := strings.LastIndexByte(s.pending, '['); i >= 0 && !strings.Contains(s.pending[i:], "]") && len(s.pending)-i < maxHolder {
		cut = i
	}
	out := s.r.Restore(s.pending[:cut])
	s.pending = s.pending[cut:]
	return out
}

// Flush returns whatever was held back.
func (s *Stream) Flush() string {
	out := s.r.Restore(s.pending)
	s.pending = ""
	return out
}
