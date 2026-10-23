package imports

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

func TestReadCSVVariants(t *testing.T) {
	utf8BOM := "\xEF\xBB\xBFФИО,Почта\r\nАйгерім Сәрсенова,aigerim@qadam.test\r\n\r\n,\r\n"
	win1251, err := charmap.Windows1251.NewEncoder().String("ФИО;Почта;Группа\nДана Ахметова;dana@qadam.test;ТПП-23-1\n")
	if err != nil {
		t.Fatal(err)
	}
	tabs := "full_name\temail\n\"Ким, Тимур\"\ttimur@qadam.test\n"
	for name, data := range map[string]string{"utf8": utf8BOM, "cp1251": win1251, "tabs": tabs} {
		tb, err := readTable("list.csv", []byte(data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(tb.rows) != 1 || len(tb.header) < 2 || tb.lines[0] != 2 {
			t.Fatalf("%s: %+v", name, tb)
		}
		index, _, err := mapHeader(tb.header, studentColumns)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		v := values(tb.rows[0], index)
		if !strings.Contains(v["email"], "@qadam.test") || v["full_name"] == "" {
			t.Fatalf("%s: %v", name, v)
		}
	}
}

func TestReadXLSX(t *testing.T) {
	f := excelize.NewFile()
	rows := [][]any{{"Email", "Full_Name", "Курс"}, {"a@qadam.test", "Әсел Нұрланқызы", 2}, {nil, nil, nil}, {"b@qadam.test", "Бауыржан Ә.", 1}}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	// The extension does not matter: XLSX is recognised by its signature.
	tb, err := readTable("students.csv", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.rows) != 2 || tb.lines[1] != 4 || tb.rows[0][2] != "2" {
		t.Fatalf("%+v", tb)
	}
}

// xlsxWithSheet builds a workbook whose first worksheet has the given rows
// as raw XML, to reproduce files that excelize itself would not write.
func xlsxWithSheet(t *testing.T, sheetData string) []byte {
	t.Helper()
	var plain bytes.Buffer
	if err := excelize.NewFile().Write(&plain); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(plain.Bytes()), int64(plain.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, file := range zr.File {
		w, err := zw.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		if file.Name == "xl/worksheets/sheet1.xml" {
			_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
				`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`+
				sheetData+`</sheetData></worksheet>`)
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(w, rc)
		_ = rc.Close()
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func inline(ref, text string) string {
	return `<c r="` + ref + `" t="inlineStr"><is><t>` + text + `</t></is></c>`
}

func TestReadXLSXLimits(t *testing.T) {
	// A row numbered far beyond the data used to make the reader step
	// through every row number in between (GO-2026-6685).
	huge := xlsxWithSheet(t, `<row r="1">`+inline("A1", "email")+`</row>`+
		`<row r="2">`+inline("A2", "a@qadam.test")+`</row>`+
		`<row r="99999999999">`+inline("A3", "x")+`</row>`)
	start := time.Now()
	if _, err := readTable("students.xlsx", huge); !apperr.Is(err, "import.too_many_rows") {
		t.Fatalf("huge row number: %v", err)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("huge row number took %v", took)
	}

	// Cells right of the header are not kept, however far right they are.
	wide := xlsxWithSheet(t, `<row r="1">`+inline("A1", "email")+inline("B1", "full_name")+`</row>`+
		`<row r="2">`+inline("A2", "a@qadam.test")+inline("B2", "Әсел")+inline("XFD2", "far")+`</row>`)
	tb, err := readTable("students.xlsx", wide)
	if err != nil || len(tb.rows) != 1 || len(tb.rows[0]) != 2 || tb.rows[0][1] != "Әсел" {
		t.Fatalf("wide row: %+v %v", tb, err)
	}

	var header strings.Builder
	for i := 1; i <= maxColumns+1; i++ {
		cell, _ := excelize.CoordinatesToCellName(i, 1)
		header.WriteString(inline(cell, "column"))
	}
	tooWide := xlsxWithSheet(t, `<row r="1">`+header.String()+`</row><row r="2">`+inline("A2", "a@qadam.test")+`</row>`)
	if _, err := readTable("students.xlsx", tooWide); !apperr.Is(err, "import.too_many_columns") {
		t.Fatalf("wide header: %v", err)
	}
}

func TestEncryptedWorkbookIsRefused(t *testing.T) {
	// Encrypted workbooks and old .xls files are OLE containers, never opened.
	ole := append([]byte(oleSignature), bytes.Repeat([]byte{0xFF}, 4096)...)
	for _, name := range []string{"students.xlsx", "students.xls", "students.csv"} {
		if _, err := readTable(name, ole); !apperr.Is(err, "import.invalid_file") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestFileErrors(t *testing.T) {
	cases := map[string]struct {
		data []byte
		code string
	}{
		"empty":     {[]byte("email,full_name\n"), "import.empty"},
		"garbage":   {[]byte("PK\x03\x04 not really a zip"), "import.invalid_file"},
		"too many":  {[]byte("email\n" + strings.Repeat("a@b.kz\n", maxRows+1)), "import.too_many_rows"},
		"bad quote": {[]byte("email,full_name\n\"a@b.kz,x\n\"\"\n,\"\n"), ""},
		"too wide":  {[]byte(strings.Repeat("column,", maxColumns) + "email\na@b.kz\n"), "import.too_many_columns"},
	}
	for name, c := range cases {
		_, err := readTable("x.csv", c.data)
		if c.code == "" {
			continue
		}
		if !apperr.Is(err, c.code) {
			t.Errorf("%s: %v", name, err)
		}
	}
	for header, code := range map[string]string{
		"email,full_name,salary": "import.unknown_columns",
		"email,email,full_name":  "import.duplicate_columns",
		"full_name,group":        "import.missing_columns",
	} {
		if _, _, err := mapHeader(strings.Split(header, ","), studentColumns); !apperr.Is(err, code) {
			t.Errorf("%s: %v", header, err)
		}
	}
	index, keys, err := mapHeader([]string{"  E-Mail* ", "Ф.И.О.", "", "Топ"}, studentColumns)
	if err != nil || len(keys) != 3 || index["group_code"] != 3 {
		t.Fatalf("aliases: %v %v %v", index, keys, err)
	}

	// Trailing empty header cells do not count, extra cells of a row are dropped.
	tb, err := readTable("x.csv", []byte("email,full_name,,,\na@b.kz,Дана,x,y,z,w\n"))
	if err != nil || len(tb.header) != 2 || len(tb.rows[0]) != 2 {
		t.Fatalf("trim: %+v %v", tb, err)
	}
}

func TestParseRoles(t *testing.T) {
	for in, want := range map[string]string{
		"":        "student",
		"Студент": "student",
		"научный руководитель; ментор": "supervisor,mentor",
		"jury, jury":                 "jury",
		"innovation_manager|partner": "innovation_manager,partner",
	} {
		got, ok := parseRoles(in)
		if !ok || strings.Join(got, ",") != want {
			t.Errorf("%q: %v %v", in, got, ok)
		}
	}
	for _, bad := range []string{"tenant_admin", "leadership", "ректор", "student,,god"} {
		if _, ok := parseRoles(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestRangeError(t *testing.T) {
	for value, want := range map[string]string{"": "", "3": "", "0": "min", "9": "max", "two": "invalid"} {
		e, bad := rangeError("course", value, 1, 7)
		if (want == "") == bad || (bad && e.Code != want) {
			t.Errorf("%q: %+v %v", value, e, bad)
		}
	}
}
