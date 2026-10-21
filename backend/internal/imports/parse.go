package imports

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// Limits of an imported file.
const (
	maxImportBytes = 10 << 20
	maxRows        = 3000
)

var errInvalidFile = apperr.New(apperr.KindValidation, "import.invalid_file",
	"the file must be CSV (UTF-8 or Windows-1251) or XLSX with headers in the first row")

// table is the content of an uploaded spreadsheet.
type table struct {
	header []string
	rows   [][]string
	// lines are the numbers of the rows in the file (the header is line 1).
	lines []int
}

func readTable(name string, data []byte) (table, error) {
	var (
		t   table
		err error
	)
	if strings.EqualFold(filepath.Ext(name), ".xlsx") || bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		t, err = readXLSX(data)
	} else {
		t, err = readCSV(data)
	}
	if err != nil {
		return t, err
	}
	if len(t.header) == 0 {
		return t, errInvalidFile
	}
	if len(t.rows) == 0 {
		return t, apperr.New(apperr.KindValidation, "import.empty", "the file has no data rows")
	}
	if len(t.rows) > maxRows {
		return t, apperr.New(apperr.KindValidation, "import.too_many_rows", "the file has too many rows").
			WithParams(map[string]any{"max": maxRows})
	}
	return t, nil
}

func blank(record []string) bool {
	for _, v := range record {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// readCSV accepts UTF-8 (with or without BOM) and Windows-1251, which Excel
// uses for "CSV" in Russian and Kazakh locales, separated by ; , or tab.
func readCSV(data []byte) (table, error) {
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	if !utf8.Valid(data) {
		decoded, err := charmap.Windows1251.NewDecoder().Bytes(data)
		if err != nil {
			return table{}, errInvalidFile
		}
		data = decoded
	}
	first, _, _ := bytes.Cut(data, []byte("\n"))
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = delimiter(first)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	var t table
	header, err := r.Read()
	if err != nil {
		return t, errInvalidFile
	}
	t.header = header
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return t, errInvalidFile
		}
		if blank(record) {
			continue
		}
		line, _ := r.FieldPos(0)
		t.rows = append(t.rows, record)
		t.lines = append(t.lines, line)
		if len(t.rows) > maxRows {
			break
		}
	}
	return t, nil
}

func delimiter(headerLine []byte) rune {
	best, count := ',', bytes.Count(headerLine, []byte(","))
	for _, d := range []rune{';', '\t'} {
		if n := bytes.Count(headerLine, []byte(string(d))); n > count {
			best, count = d, n
		}
	}
	return best
}

func readXLSX(data []byte) (table, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit: 64 << 20, UnzipXMLSizeLimit: 32 << 20,
	})
	if err != nil {
		return table{}, errInvalidFile
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return table{}, errInvalidFile
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil || len(rows) == 0 {
		return table{}, errInvalidFile
	}
	t := table{header: rows[0]}
	for i, record := range rows[1:] {
		if blank(record) {
			continue
		}
		t.rows = append(t.rows, record)
		t.lines = append(t.lines, i+2)
		if len(t.rows) > maxRows {
			break
		}
	}
	return t, nil
}

// column is a recognised column with the header spellings accepted for it.
type column struct {
	key      string
	required bool
	aliases  []string
}

func normalizeHeader(h string) string {
	h = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF")))
	h = strings.ReplaceAll(h, "ё", "е")
	h = strings.Trim(h, "* ")
	return strings.Join(strings.Fields(h), " ")
}

// mapHeader finds the index of every known column. Unknown columns are an
// error: silently dropping data the administrator meant to import is worse.
func mapHeader(header []string, columns []column) (map[string]int, []string, error) {
	byAlias := map[string]string{}
	for _, c := range columns {
		byAlias[c.key] = c.key
		for _, a := range c.aliases {
			byAlias[normalizeHeader(a)] = c.key
		}
	}
	index := map[string]int{}
	var unknown []string
	for i, h := range header {
		n := normalizeHeader(h)
		if n == "" {
			continue
		}
		key, ok := byAlias[n]
		if !ok {
			unknown = append(unknown, strings.TrimSpace(h))
			continue
		}
		if _, dup := index[key]; dup {
			return nil, nil, apperr.New(apperr.KindValidation, "import.duplicate_columns", "a column appears twice").
				WithParams(map[string]any{"columns": strings.TrimSpace(h)})
		}
		index[key] = i
	}
	if len(unknown) > 0 {
		return nil, nil, apperr.New(apperr.KindValidation, "import.unknown_columns", "the file has columns that cannot be imported").
			WithParams(map[string]any{"columns": strings.Join(unknown, ", ")})
	}
	var missing, keys []string
	for _, c := range columns {
		if _, ok := index[c.key]; ok {
			keys = append(keys, c.key)
		} else if c.required {
			missing = append(missing, c.key)
		}
	}
	if len(missing) > 0 {
		return nil, nil, apperr.New(apperr.KindValidation, "import.missing_columns", "required columns are missing").
			WithParams(map[string]any{"columns": strings.Join(missing, ", ")})
	}
	return index, keys, nil
}

func values(record []string, index map[string]int) map[string]string {
	out := make(map[string]string, len(index))
	for key, i := range index {
		v := ""
		if i < len(record) {
			v = strings.Join(strings.Fields(record[i]), " ")
		}
		out[key] = v
	}
	return out
}
