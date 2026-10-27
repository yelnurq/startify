package knowledge

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	"golang.org/x/text/encoding/charmap"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

var errUnsupported = apperr.New(apperr.KindValidation, "knowledge.unsupported_file",
	"upload a PDF, DOCX, TXT or Markdown file")

var errNoText = apperr.New(apperr.KindValidation, "knowledge.no_text",
	"no text could be extracted (a scanned PDF needs text recognition first)")

// errTooLong: a small compressed file can unpack into far more text than a
// knowledge document may hold (Н-28).
var errTooLong = apperr.New(apperr.KindValidation, "knowledge.text_too_long",
	"the document has too much text; split it into several documents")

type format int

const (
	formatUnknown format = iota
	formatPDF
	formatDOCX
	formatText
)

func formatOf(name string, data []byte) format {
	switch ext := strings.ToLower(filepath.Ext(name)); {
	case ext == ".pdf" || bytes.HasPrefix(data, []byte("%PDF")):
		return formatPDF
	case ext == ".docx":
		return formatDOCX
	case ext == ".txt" || ext == ".md" || ext == ".markdown":
		return formatText
	}
	return formatUnknown
}

// supported reports whether Extract reads files of this kind.
func supported(name string, data []byte) bool { return formatOf(name, data) != formatUnknown }

// Extract returns the text of an uploaded document, at most maxTextBytes.
func Extract(name string, data []byte) (string, error) {
	var (
		text string
		err  error
	)
	switch formatOf(name, data) {
	case formatPDF:
		text, err = extractPDF(data)
	case formatDOCX:
		text, err = extractDOCX(data)
	case formatText:
		text = decodeText(data)
	default:
		return "", errUnsupported
	}
	if err != nil {
		return "", err
	}
	if len(text) > maxTextBytes {
		return "", errTooLong
	}
	text = normalize(text)
	if utf8.RuneCountInString(strings.TrimSpace(text)) < 20 {
		return "", errNoText
	}
	return text, nil
}

// decodeText reads UTF-8 (with or without BOM) or Windows-1251.
func decodeText(data []byte) string {
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	if utf8.Valid(data) {
		return string(data)
	}
	decoded, err := charmap.Windows1251.NewDecoder().Bytes(data)
	if err != nil {
		return string(bytes.ToValidUTF8(data, nil))
	}
	return string(decoded)
}

func extractPDF(data []byte) (text string, err error) {
	// The parser panics on some malformed files; a bad upload must not crash the worker.
	defer func() {
		if recover() != nil {
			text, err = "", errNoText
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errUnsupported
	}
	var b strings.Builder
	fonts := map[string]*pdf.Font{}
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		for _, name := range page.Fonts() {
			if _, ok := fonts[name]; !ok {
				f := page.Font(name)
				fonts[name] = &f
			}
		}
		t, err := page.GetPlainText(fonts)
		if err != nil {
			continue
		}
		b.WriteString(t)
		b.WriteString("\n\n")
		if b.Len() > maxTextBytes {
			return "", errTooLong
		}
	}
	return b.String(), nil
}

// extractDOCX reads the paragraphs of word/document.xml.
func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errUnsupported
	}
	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
		}
	}
	if doc == nil || doc.UncompressedSize64 > 50<<20 {
		return "", errUnsupported
	}
	rc, err := doc.Open()
	if err != nil {
		return "", errUnsupported
	}
	defer func() { _ = rc.Close() }()
	dec := xml.NewDecoder(io.LimitReader(rc, 50<<20))
	var b strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", errUnsupported
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				b.WriteByte('\t')
			case "br":
				b.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				b.WriteString("\n\n")
			}
		case xml.CharData:
			if inText {
				b.Write(t)
				if b.Len() > maxTextBytes {
					return "", errTooLong
				}
			}
		}
	}
	return b.String(), nil
}

// normalize collapses whitespace inside lines and blank-line runs.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, l := range lines {
		l = strings.Join(strings.Fields(l), " ")
		if l == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
