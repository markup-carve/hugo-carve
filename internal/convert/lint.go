package convert

import (
	"strings"
	"unicode/utf8"

	carve "github.com/markup-carve/carve-go"
)

// Finding is one lint finding, positioned in the source file as written.
type Finding struct {
	Line    int
	Column  int
	Rule    string
	Message string
}

// Lint runs the engine's lint pass over a content file's Carve body.
//
// Front matter is split off first, the same way ConvertWithOptions does, since
// TOML and JSON front matter are not Carve. Lines are shifted back by the
// front matter's length so a finding points at the line in the file.
func Lint(source string, opts Options) ([]Finding, error) {
	_, body := splitFrontMatter(source)
	head := source[:len(source)-len(body)]
	offset := strings.Count(head, "\n")
	// JSON front matter can end mid-line, so the body's first line starts
	// that many codepoints in.
	shift := utf8.RuneCountInString(head[strings.LastIndex(head, "\n")+1:])

	found, err := carve.LintWithOptions(body, carve.LintOptions{Bundle: opts.Extensions || opts.Static})
	if err != nil {
		return nil, err
	}
	findings := make([]Finding, 0, len(found))
	for _, f := range found {
		column := f.Column
		if f.Line == 1 {
			column += shift
		}
		findings = append(findings, Finding{
			Line:    f.Line + offset,
			Column:  column,
			Rule:    f.Rule,
			Message: f.Message,
		})
	}
	return findings, nil
}
