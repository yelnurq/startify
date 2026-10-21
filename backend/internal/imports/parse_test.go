package imports

import (
	"bytes"
	"strings"
	"testing"

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

func TestFileErrors(t *testing.T) {
	cases := map[string]struct {
		data []byte
		code string
	}{
		"empty":     {[]byte("email,full_name\n"), "import.empty"},
		"garbage":   {[]byte("PK\x03\x04 not really a zip"), "import.invalid_file"},
		"too many":  {[]byte("email\n" + strings.Repeat("a@b.kz\n", maxRows+1)), "import.too_many_rows"},
		"bad quote": {[]byte("email,full_name\n\"a@b.kz,x\n\"\"\n,\"\n"), ""},
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
