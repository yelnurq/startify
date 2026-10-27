package knowledge

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Chunk is a passage of a document with the heading it falls under.
type Chunk struct {
	Heading string
	Text    string
}

// Passage sizes in characters: long enough to carry a rule with its
// context, short enough that a search hit is precise.
const (
	targetChars = 900
	maxChars    = 1400
)

// Split cuts a document into passages along paragraphs, never splitting a
// sentence unless it is itself too long, and remembers the latest heading.
func Split(text string) []Chunk {
	var out []Chunk
	heading := ""
	var cur []string
	size := 0
	flush := func() {
		if len(cur) == 0 {
			return
		}
		out = append(out, Chunk{Heading: heading, Text: strings.Join(cur, "\n\n")})
		cur, size = nil, 0
	}
	for _, para := range strings.Split(text, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if isHeading(para) {
			flush()
			heading = strings.TrimSpace(strings.TrimLeft(para, "#"))
			continue
		}
		for _, piece := range pieces(para) {
			n := utf8.RuneCountInString(piece)
			if size > 0 && size+n > targetChars {
				flush()
			}
			cur = append(cur, piece)
			size += n
		}
	}
	flush()
	return out
}

// isHeading recognises Markdown headings and short title lines such as
// "Раздел 3. Порядок подачи заявок".
func isHeading(p string) bool {
	if strings.HasPrefix(p, "#") {
		return true
	}
	if strings.Contains(p, "\n") || utf8.RuneCountInString(p) > 90 {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(p)
	if strings.ContainsRune(".,;:!?»\")", last) {
		return false
	}
	first, _ := utf8.DecodeRuneInString(p)
	return unicode.IsUpper(first) || unicode.IsDigit(first)
}

// pieces splits a paragraph longer than maxChars at sentence ends.
func pieces(p string) []string {
	if utf8.RuneCountInString(p) <= maxChars {
		return []string{p}
	}
	var out []string
	var b strings.Builder
	runes := []rune(p)
	for i, r := range runes {
		b.WriteRune(r)
		end := (r == '.' || r == '!' || r == '?') && (i+1 == len(runes) || unicode.IsSpace(runes[i+1]))
		if (end && utf8.RuneCountInString(b.String()) >= targetChars) || utf8.RuneCountInString(b.String()) >= maxChars {
			out = append(out, strings.TrimSpace(b.String()))
			b.Reset()
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		out = append(out, s)
	}
	return out
}
