package incubator

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// Brand is how generated documents present the university.
type Brand struct {
	University string `json:"university"`
	Color      string `json:"color"`
}

// hexColor returns the brand colour as RRGGBB, or the Qadam cyan.
func (b Brand) hexColor() string {
	c := strings.TrimPrefix(b.Color, "#")
	if len(c) != 6 {
		return "00A3C4"
	}
	for _, r := range c {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return "00A3C4"
		}
	}
	return strings.ToUpper(c)
}

const (
	nsA       = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
	nsR       = `xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	nsP       = `xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"
	relsNS    = `xmlns="http://schemas.openxmlformats.org/package/2006/relationships"`
	relBase   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"

	// Slide size 16:9 in EMU.
	slideW = 12192000
	slideH = 6858000
	inch   = 914400
)

func esc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

var langTags = map[string]string{"kk": "kk-KZ", "ru": "ru-RU", "en": "en-US"}

const emptyGroup = `<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
	`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`

// deckWriter accumulates the parts of the package.
type deckWriter struct {
	lang  string
	color string
	ids   int
}

func (d *deckWriter) id() int { d.ids++; return d.ids }

func (d *deckWriter) rect(x, y, w, h int, color string) string {
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Shape %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>`+
		`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`+
		`<a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:ln><a:noFill/></a:ln></p:spPr></p:sp>`,
		d.id(), d.ids, x, y, w, h, color)
}

// run is a piece of formatted text.
func (d *deckWriter) run(text string, size int, bold bool, color string) string {
	b := ""
	if bold {
		b = ` b="1"`
	}
	return fmt.Sprintf(`<a:r><a:rPr lang="%s" sz="%d"%s dirty="0"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>`+
		`<a:latin typeface="Arial"/><a:cs typeface="Arial"/></a:rPr><a:t>%s</a:t></a:r>`, d.lang, size, b, color, esc(text))
}

// textBox places text across the slide between the side margins.
func (d *deckWriter) textBox(y, h int, anchor, paragraphs string) string {
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Text %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>`+
		`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/></p:spPr>`+
		`<p:txBody><a:bodyPr wrap="square" lIns="0" rIns="0" anchor="%s"><a:normAutofit/></a:bodyPr><a:lstStyle/>%s</p:txBody></p:sp>`,
		d.id(), d.ids, inch, y, slideW-2*inch, h, anchor, paragraphs)
}

func (d *deckWriter) slide(shapes string) string {
	d.ids = 1
	return xmlHeader + `<p:sld ` + nsA + ` ` + nsR + ` ` + nsP + `><p:cSld><p:spTree>` + emptyGroup + shapes +
		`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`
}

const (
	ink   = "101828"
	muted = "475467"
	faint = "98A2B3"
)

func (d *deckWriter) titleSlide(title, tagline, footer string) string {
	shapes := d.rect(0, 0, slideW, slideH/40, d.color)
	shapes += d.textBox(2*inch, int(1.6*inch), "b",
		`<a:p>`+d.run(title, 5400, true, ink)+`</a:p>`)
	if tagline != "" {
		shapes += d.textBox(int(3.95*inch), int(1.3*inch), "t",
			`<a:p>`+d.run(tagline, 2400, false, muted)+`</a:p>`)
	}
	shapes += d.rect(inch, int(3.72*inch), int(1.2*inch), inch/16, d.color)
	shapes += d.textBox(slideH-int(0.9*inch), int(0.5*inch), "ctr",
		`<a:p>`+d.run(footer, 1200, false, faint)+`</a:p>`)
	return d.slide(shapes)
}

func (d *deckWriter) contentSlide(s Slide, number int, footer string) string {
	shapes := d.rect(0, 0, slideW, slideH/80, d.color)
	shapes += d.textBox(int(0.55*inch), int(1.1*inch), "b",
		`<a:p>`+d.run(s.Title, 3600, true, ink)+`</a:p>`)
	var ps strings.Builder
	for _, b := range s.Bullets {
		ps.WriteString(`<a:p><a:pPr marL="342900" indent="-342900"><a:spcAft><a:spcPts val="1400"/></a:spcAft>` +
			`<a:buClr><a:srgbClr val="` + d.color + `"/></a:buClr><a:buFont typeface="Arial"/><a:buChar char="•"/></a:pPr>` +
			d.run(b, 2400, false, ink) + `</a:p>`)
	}
	if ps.Len() == 0 {
		ps.WriteString(`<a:p><a:endParaRPr lang="` + d.lang + `"/></a:p>`)
	}
	shapes += d.textBox(int(1.9*inch), int(4.4*inch), "t", ps.String())
	shapes += d.textBox(slideH-int(0.7*inch), int(0.4*inch), "ctr",
		`<a:p><a:pPr algn="l"/>`+d.run(fmt.Sprintf("%s · %d", footer, number), 1100, false, faint)+`</a:p>`)
	return d.slide(shapes)
}

// BuildDeck writes the pitch deck as a PowerPoint presentation. The first
// slide is the title slide; the rest come from the content.
func BuildDeck(title, tagline, locale string, slides []Slide, brand Brand) ([]byte, error) {
	lang := langTags[locale]
	if lang == "" {
		lang = "ru-RU"
	}
	d := &deckWriter{lang: lang, color: brand.hexColor()}
	footer := "Qadam"
	if brand.University != "" {
		footer = brand.University + " · Qadam"
	}
	parts := []string{d.titleSlide(title, tagline, footer)}
	for i, s := range slides {
		if i == 0 && strings.TrimSpace(s.Title) == strings.TrimSpace(title) {
			continue // the model's own title slide is replaced by ours
		}
		parts = append(parts, d.contentSlide(s, len(parts)+1, footer))
	}

	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	add := func(name, content string) error {
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(content))
		return err
	}
	var overrides, presRels, sldIDs strings.Builder
	for i := range parts {
		n := i + 1
		fmt.Fprintf(&overrides, `<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, n)
		fmt.Fprintf(&presRels, `<Relationship Id="rId%d" Type="%sslide" Target="slides/slide%d.xml"/>`, n+2, relBase, n)
		fmt.Fprintf(&sldIDs, `<p:sldId id="%d" r:id="rId%d"/>`, 255+n, n+2)
	}
	files := []struct{ name, content string }{
		{"[Content_Types].xml", xmlHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>` +
			`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>` +
			`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
			`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
			overrides.String() +
			`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
			`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
			`</Types>`},
		{"_rels/.rels", xmlHeader + `<Relationships ` + relsNS + `>` +
			`<Relationship Id="rId1" Type="` + relBase + `officeDocument" Target="ppt/presentation.xml"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
			`<Relationship Id="rId3" Type="` + relBase + `extended-properties" Target="docProps/app.xml"/>` +
			`</Relationships>`},
		{"docProps/core.xml", xmlHeader + `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
			`xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>` + esc(title) + `</dc:title><dc:creator>Qadam</dc:creator></cp:coreProperties>`},
		{"docProps/app.xml", xmlHeader + `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">` +
			fmt.Sprintf(`<Application>Qadam</Application><Slides>%d</Slides></Properties>`, len(parts))},
		{"ppt/presentation.xml", xmlHeader + `<p:presentation ` + nsA + ` ` + nsR + ` ` + nsP + `>` +
			`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
			`<p:sldIdLst>` + sldIDs.String() + `</p:sldIdLst>` +
			fmt.Sprintf(`<p:sldSz cx="%d" cy="%d"/><p:notesSz cx="6858000" cy="9144000"/>`, slideW, slideH) +
			`<p:defaultTextStyle><a:lvl1pPr><a:defRPr lang="` + lang + `"/></a:lvl1pPr></p:defaultTextStyle></p:presentation>`},
		{"ppt/_rels/presentation.xml.rels", xmlHeader + `<Relationships ` + relsNS + `>` +
			`<Relationship Id="rId1" Type="` + relBase + `slideMaster" Target="slideMasters/slideMaster1.xml"/>` +
			`<Relationship Id="rId2" Type="` + relBase + `theme" Target="theme/theme1.xml"/>` +
			presRels.String() + `</Relationships>`},
		{"ppt/slideMasters/slideMaster1.xml", xmlHeader + `<p:sldMaster ` + nsA + ` ` + nsR + ` ` + nsP + `>` +
			`<p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>` +
			`<p:spTree>` + emptyGroup + `</p:spTree></p:cSld>` +
			`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
			`<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>` +
			`<p:txStyles><p:titleStyle><a:lvl1pPr><a:defRPr sz="3600"/></a:lvl1pPr></p:titleStyle>` +
			`<p:bodyStyle><a:lvl1pPr><a:defRPr sz="2400"/></a:lvl1pPr></p:bodyStyle>` +
			`<p:otherStyle><a:lvl1pPr><a:defRPr sz="1800"/></a:lvl1pPr></p:otherStyle></p:txStyles></p:sldMaster>`},
		{"ppt/slideMasters/_rels/slideMaster1.xml.rels", xmlHeader + `<Relationships ` + relsNS + `>` +
			`<Relationship Id="rId1" Type="` + relBase + `slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
			`<Relationship Id="rId2" Type="` + relBase + `theme" Target="../theme/theme1.xml"/></Relationships>`},
		{"ppt/slideLayouts/slideLayout1.xml", xmlHeader + `<p:sldLayout ` + nsA + ` ` + nsR + ` ` + nsP + ` type="blank" preserve="1">` +
			`<p:cSld name="Blank"><p:spTree>` + emptyGroup + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`},
		{"ppt/slideLayouts/_rels/slideLayout1.xml.rels", xmlHeader + `<Relationships ` + relsNS + `>` +
			`<Relationship Id="rId1" Type="` + relBase + `slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`},
		{"ppt/theme/theme1.xml", theme(d.color)},
	}
	for _, f := range files {
		if err := add(f.name, f.content); err != nil {
			return nil, fmt.Errorf("incubator: pptx: %w", err)
		}
	}
	slideRels := xmlHeader + `<Relationships ` + relsNS + `><Relationship Id="rId1" Type="` + relBase +
		`slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`
	for i, p := range parts {
		if err := add(fmt.Sprintf("ppt/slides/slide%d.xml", i+1), p); err != nil {
			return nil, fmt.Errorf("incubator: pptx: %w", err)
		}
		if err := add(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1), slideRels); err != nil {
			return nil, fmt.Errorf("incubator: pptx: %w", err)
		}
	}
	if err := z.Close(); err != nil {
		return nil, fmt.Errorf("incubator: pptx: %w", err)
	}
	return buf.Bytes(), nil
}

func theme(accent string) string {
	solid := `<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>`
	line := `<a:ln w="9525">` + solid + `</a:ln>`
	effect := `<a:effectStyle><a:effectLst/></a:effectStyle>`
	font := `<a:latin typeface="Arial"/><a:ea typeface=""/><a:cs typeface="Arial"/>`
	color := func(name, rgb string) string {
		return `<a:` + name + `><a:srgbClr val="` + rgb + `"/></a:` + name + `>`
	}
	return xmlHeader + `<a:theme ` + nsA + ` name="Qadam"><a:themeElements>` +
		`<a:clrScheme name="Qadam">` + color("dk1", ink) + color("lt1", "FFFFFF") + color("dk2", "1D2939") + color("lt2", "F2F4F7") +
		color("accent1", accent) + color("accent2", "7C3AED") + color("accent3", "16A34A") + color("accent4", "F59E0B") +
		color("accent5", "DC2626") + color("accent6", "475467") + color("hlink", accent) + color("folHlink", "7C3AED") + `</a:clrScheme>` +
		`<a:fontScheme name="Qadam"><a:majorFont>` + font + `</a:majorFont><a:minorFont>` + font + `</a:minorFont></a:fontScheme>` +
		`<a:fmtScheme name="Qadam"><a:fillStyleLst>` + solid + solid + solid + `</a:fillStyleLst>` +
		`<a:lnStyleLst>` + line + line + line + `</a:lnStyleLst>` +
		`<a:effectStyleLst>` + effect + effect + effect + `</a:effectStyleLst>` +
		`<a:bgFillStyleLst>` + solid + solid + solid + `</a:bgFillStyleLst></a:fmtScheme>` +
		`</a:themeElements></a:theme>`
}
