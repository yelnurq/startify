package incubator

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestBuildDeckIsAWellFormedPresentation(t *testing.T) {
	slides := []Slide{
		{Title: "AgroScan", Bullets: []string{"ignored: replaced by the title slide"}},
		{Title: "Мәселе", Bullets: []string{"Фермерлер бидай ауруларын кеш байқайды", "Шығын — өнімнің 20%-ы <дейін> & одан да көп"}},
		{Title: "Команда", Bullets: nil},
	}
	data, err := BuildDeck("AgroScan", "Диагностика по фото листа", "kk", slides, Brand{University: "ҚазТБУ", Color: "#00a3c4"})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range z.File {
		names[f.Name] = true
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, _ := io.ReadAll(r)
		_ = r.Close()
		// Every part parses as XML.
		dec := xml.NewDecoder(bytes.NewReader(content))
		for {
			if _, err := dec.Token(); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				t.Fatalf("%s: %v", f.Name, err)
			}
		}
		if f.Name == "ppt/slides/slide2.xml" {
			s := string(content)
			if !strings.Contains(s, "&lt;дейін&gt; &amp; одан") || !strings.Contains(s, `lang="kk-KZ"`) || !strings.Contains(s, `val="00A3C4"`) {
				t.Fatalf("slide 2: %s", s)
			}
		}
		if f.Name == "ppt/presentation.xml" && strings.Count(string(content), "<p:sldId ") != 3 {
			t.Fatalf("slides: %s", content)
		}
	}
	for _, want := range []string{
		"[Content_Types].xml", "_rels/.rels", "ppt/presentation.xml", "ppt/slideMasters/slideMaster1.xml",
		"ppt/slideLayouts/slideLayout1.xml", "ppt/theme/theme1.xml", "ppt/slides/slide3.xml", "ppt/slides/_rels/slide3.xml.rels",
	} {
		if !names[want] {
			t.Errorf("missing %s", want)
		}
	}
	if names["ppt/slides/slide4.xml"] {
		t.Fatal("the model's title slide must be replaced, not added")
	}
	if (Brand{Color: "red"}).hexColor() != "00A3C4" {
		t.Fatal("invalid colours fall back to the Qadam cyan")
	}
}
