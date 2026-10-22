// Package prompts holds the system prompts as versioned templates. Every
// template starts with `{{- /* prompt: <name>; version: <n> */ -}}`; the
// "name@version" string is stored with each result, so an answer can always
// be traced to the exact wording that produced it.
package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"sync"
	"text/template"
)

//go:embed *.tmpl
var files embed.FS

var header = regexp.MustCompile(`^\{\{-?\s*/\*\s*prompt:\s*([a-z_]+);\s*version:\s*(\d+)\s*\*/\s*-?\}\}`)

var languages = map[string]string{"kk": "Kazakh", "ru": "Russian", "en": "English"}

// quote neutralises angle brackets in user data, so it cannot close or open
// the tags that separate data from instructions.
var quote = strings.NewReplacer("<", "‹", ">", "›").Replace

var funcs = template.FuncMap{
	"quote": quote,
	"language": func(locale string) string {
		if l, ok := languages[locale]; ok {
			return l
		}
		return "Russian"
	},
}

type prompt struct {
	version string
	tmpl    *template.Template
}

var load = sync.OnceValues(func() (map[string]prompt, error) {
	out := map[string]prompt{}
	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := files.ReadFile(path)
		if err != nil {
			return err
		}
		m := header.FindSubmatch(raw)
		if m == nil {
			return fmt.Errorf("prompts: %s has no version header", path)
		}
		name := string(m[1])
		if name+".tmpl" != path {
			return fmt.Errorf("prompts: %s declares the name %q", path, name)
		}
		t, err := template.New(name).Funcs(funcs).Option("missingkey=error").Parse(string(raw))
		if err != nil {
			return fmt.Errorf("prompts: %s: %w", path, err)
		}
		out[name] = prompt{version: name + "@" + string(m[2]), tmpl: t}
		return nil
	})
	return out, err
})

// Render fills a template and returns the text with its version.
func Render(name string, data any) (string, string, error) {
	all, err := load()
	if err != nil {
		return "", "", err
	}
	p, ok := all[name]
	if !ok {
		return "", "", fmt.Errorf("prompts: unknown prompt %q", name)
	}
	var buf bytes.Buffer
	if err := p.tmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("prompts: render %s: %w", name, err)
	}
	return strings.TrimSpace(buf.String()), p.version, nil
}

// Source is a document excerpt given to the model.
type Source struct {
	Title string
	Text  string
}

// Mentor is the data of the mentor prompt.
type Mentor struct {
	StepName     string
	ProjectTitle string
	// Mode is coach or coauthor.
	Mode        string
	Locale      string
	ProjectData string
	Knowledge   []Source
}

// Criterion is a rubric line of the evaluation prompt.
type Criterion struct {
	Key    string
	Title  string
	Weight int
}

// Evaluation is the data of the evaluation prompt.
type Evaluation struct {
	Locale      string
	Rubric      []Criterion
	ProjectData string
}

// ConnectionCheck is the data of the administrator's connection check.
type ConnectionCheck struct {
	Tenant string
	Locale string
}
