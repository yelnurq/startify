package imports

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"path/filepath"
	"slices"
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
	// maxColumns bounds the header; the known columns are far fewer.
	maxColumns = 50
	// maxSheetRows bounds the rows of a worksheet looked at, blank ones
	// included: a row numbered far below the data makes the reader step
	// through every row number in between.
	maxSheetRows = 100 * maxRows
	// Unpacked size of the whole workbook and of one XML part kept in memory.
	xlsxUnzipLimit   = 64 << 20
	xlsxXMLPartLimit = 32 << 20
	zipSignature     = "PK\x03\x04"
	// oleSignature starts encrypted workbooks and old .xls files.
	oleSignature = "\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1"
)

var errInvalidFile = apperr.New(apperr.KindValidation, "import.invalid_file",
	"the file must be CSV (UTF-8 or Windows-1251) or XLSX with headers in the first row")

func errTooManyRows() error {
	return apperr.New(apperr.KindValidation, "import.too_many_rows", "the file has too many rows").
		WithParams(map[string]any{"max": maxRows})
}

// table is the content of an uploaded spreadsheet.
type table struct {
	header []string
	rows   [][]string
	// lines are the numbers of the rows in the file (the header is line 1).
	lines []int
}

// setHeader keeps the header row; a header wider than maxColumns is refused.
func (t *table) setHeader(header []string) error {
	for len(header) > 0 && strings.TrimSpace(header[len(header)-1]) == "" {
		header = header[:len(header)-1]
	}
	if len(header) > maxColumns {
		return apperr.New(apperr.KindValidation, "import.too_many_columns", "the file has too many columns").
			WithParams(map[string]any{"max": maxColumns})
	}
	t.header = header
	return nil
}

// add keeps a data row. Cells right of the header have no column to go to,
// so they are not kept (a copy lets the rest of a very wide row go).
func (t *table) add(record []string, line int) {
	if len(record) > len(t.header) {
		record = slices.Clone(record[:len(t.header)])
	}
	t.rows = append(t.rows, record)
	t.lines = append(t.lines, line)
}

func readTable(name string, data []byte) (table, error) {
	var (
		t   table
		err error
	)
	if strings.EqualFold(filepath.Ext(name), ".xlsx") || bytes.HasPrefix(data, []byte(zipSignature)) || bytes.HasPrefix(data, []byte(oleSignature)) {
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
		return t, errTooManyRows()
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
	if err := t.setHeader(header); err != nil {
		return t, err
	}
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
		t.add(record, line)
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

// readXLSX streams the first worksheet row by row and stops at the first
// limit it reaches. excelize v2.11.0 has known flaws on crafted files
// (GO-2026-6684, -6685, -6688, -6689; fixed only after the release, see
// docs/BACKLOG.md), hence the guards around it:
//   - only a ZIP archive is opened: an encrypted workbook is an OLE container
//     whose decryption can panic or allocate without bound;
//   - the unpacked size is capped, and large XML parts go to temporary files;
//   - rows are read by the iterator with a cap on the rows looked at, instead
//     of GetRows, which loops through every row number up to the last one;
//   - formulas are never evaluated, only their cached values are read;
//   - a panic inside excelize means a bad file, not a failed server.
func readXLSX(data []byte) (t table, err error) {
	if !bytes.HasPrefix(data, []byte(zipSignature)) {
		return table{}, errInvalidFile
	}
	defer func() {
		if recover() != nil {
			t, err = table{}, errInvalidFile
		}
	}()
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit: xlsxUnzipLimit, UnzipXMLSizeLimit: xlsxXMLPartLimit,
	})
	if err != nil {
		return table{}, errInvalidFile
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return table{}, errInvalidFile
	}
	rows, err := f.Rows(sheets[0])
	if err != nil {
		return table{}, errInvalidFile
	}
	defer func() { _ = rows.Close() }()
	for line := 1; rows.Next(); line++ {
		if line > maxSheetRows {
			return table{}, errTooManyRows()
		}
		record, err := rows.Columns()
		if err != nil {
			return table{}, errInvalidFile
		}
		if line == 1 {
			if err := t.setHeader(record); err != nil {
				return table{}, err
			}
			if len(t.header) == 0 {
				return table{}, errInvalidFile
			}
			continue
		}
		if blank(record) {
			continue
		}
		t.add(record, line)
		if len(t.rows) > maxRows {
			break
		}
	}
	if rows.Error() != nil {
		return table{}, errInvalidFile
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
