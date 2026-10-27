package knowledge

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

const regulation = `# Положение о конкурсе стартап-проектов

Раздел 1. Общие положения

Конкурс проводится среди студентов и магистрантов КазУТБ. Цель конкурса — поддержка проектов с потенциалом коммерциализации.

Раздел 2. Порядок подачи заявок

Заявки принимаются на платформе Qadam до 1 декабря. К заявке прикладываются питч-дек и паспорт стартапа.

Команда должна состоять не менее чем из двух человек.`

func TestSplitKeepsHeadings(t *testing.T) {
	chunks := Split(normalize(regulation))
	if len(chunks) != 2 {
		t.Fatalf("chunks: %+v", chunks)
	}
	if chunks[0].Heading != "Раздел 1. Общие положения" || !strings.Contains(chunks[1].Text, "до 1 декабря") ||
		chunks[1].Heading != "Раздел 2. Порядок подачи заявок" {
		t.Fatalf("headings: %+v", chunks)
	}

	long := strings.Repeat("Каждое предложение этого абзаца описывает требования к заявке. ", 80)
	for _, c := range Split(long) {
		if n := utf8.RuneCountInString(c.Text); n > maxChars {
			t.Fatalf("passage of %d characters", n)
		}
	}
}

func docx(t *testing.T, paragraphs ...string) []byte {
	t.Helper()
	var body strings.Builder
	for _, p := range paragraphs {
		body.WriteString(`<w:p><w:r><w:t xml:space="preserve">` + p + `</w:t></w:r></w:p>`)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
		body.String() + `</w:body></w:document>`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtract(t *testing.T) {
	text, err := Extract("Положение.docx", docx(t, "Грант для молодых учёных", "Сумма гранта — до 5 млн тенге на проект."))
	if err != nil || text != "Грант для молодых учёных\n\nСумма гранта — до 5 млн тенге на проект." {
		t.Fatalf("docx: %q %v", text, err)
	}
	cp, _ := charmap.Windows1251.NewEncoder().Bytes([]byte("Методичка по защите проекта: структура и сроки сдачи."))
	if text, err := Extract("guide.txt", cp); err != nil || !strings.HasPrefix(text, "Методичка") {
		t.Fatalf("cp1251: %q %v", text, err)
	}
	if text, err := Extract("rules.md", []byte("\xEF\xBB\xBF# Правила\n\n\n\nСтуденты   подают   заявки онлайн через платформу.")); err != nil ||
		text != "# Правила\n\nСтуденты подают заявки онлайн через платформу." {
		t.Fatalf("md: %q %v", text, err)
	}
	for name, data := range map[string][]byte{"photo.png": {0x89, 'P', 'N', 'G'}, "broken.docx": []byte("not a zip"), "empty.txt": []byte("  ")} {
		if _, err := Extract(name, data); err == nil {
			t.Errorf("%s accepted", name)
		} else if _, ok := apperr.As(err); !ok {
			t.Errorf("%s: %v is not a user-facing error", name, err)
		}
	}
	if _, err := Extract("scan.pdf", []byte("%PDF-1.4 garbage")); err == nil {
		t.Fatal("a broken PDF must be rejected, not crash")
	}
}

func TestVectorLiteral(t *testing.T) {
	if got := vectorLiteral([]float32{0.5, -1, 0.0001}); got != "[0.5,-1,0.0001]" {
		t.Fatal(got)
	}
}

// Indexing time grows with the document but stays below River's rescue of
// stuck jobs (С-17).
func TestIndexTimeoutGrowsWithTheDocument(t *testing.T) {
	small := indexTimeout(len(Split(regulation)))
	large := indexTimeout(len(Split(strings.Repeat("Каждое предложение этого абзаца описывает требования к заявке. ", 4000))))
	huge := indexTimeout(10_000)
	if small != indexBase+indexPerBatch || large <= small || huge != indexMax {
		t.Fatalf("small %s, large %s, huge %s", small, large, huge)
	}
}

// Extracted text has the same limit as pasted text (Н-28).
func TestExtractedTextIsLimited(t *testing.T) {
	big := strings.Repeat("Пункт положения о конкурсе. ", maxTextBytes/20)
	if _, err := Extract("big.docx", docx(t, big)); !apperr.Is(err, "knowledge.text_too_long") {
		t.Fatalf("docx: %v", err)
	}
	if _, err := Extract("big.md", []byte(big)); !apperr.Is(err, "knowledge.text_too_long") {
		t.Fatalf("md: %v", err)
	}
	if !supported("rules.MD", nil) || !supported("x.bin", []byte("%PDF-1.7")) || supported("photo.png", nil) {
		t.Fatal("supported formats")
	}
}
