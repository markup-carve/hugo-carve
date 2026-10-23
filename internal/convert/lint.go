package convert

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	carve "github.com/markup-carve/carve-go"
)

// Finding is one lint finding, positioned in the file it arose in.
type Finding struct {
	// File is the included file the finding arose in, relative to the include
	// root and slash-separated. Empty for the page's own findings.
	File       string
	Line       int
	Column     int
	Rule       string
	Message    string
	IncludedBy []Site
}

// Site is one include directive in a finding's chain, the page first.
type Site struct {
	File   string
	Line   int
	Column int
}

// Lint runs the engine's lint pass over a content file's Carve body.
//
// Front matter is split off first, the same way ConvertWithOptions does, since
// TOML and JSON front matter are not Carve. Positions in the page itself are
// shifted back by the front matter so they point at the line in the file.
//
// With Includes on and Safe off, the pass also lints every included Carve
// file, as RenderWithIncludes would expand them. Safe reads no files, so
// neither does its lint pass.
func Lint(source string, opts Options) ([]Finding, error) {
	_, body := splitFrontMatter(source)
	head := source[:len(source)-len(body)]
	offset := strings.Count(head, "\n")
	// JSON front matter can end mid-line, so the body's first line starts
	// that many codepoints in.
	shift := utf8.RuneCountInString(head[strings.LastIndex(head, "\n")+1:])
	position := func(line, column int) (int, int) {
		if line == 1 {
			column += shift
		}
		return line + offset, column
	}

	lintOpts := carve.LintOptions{Bundle: opts.Extensions || opts.Static}
	page := ""
	if opts.Includes && !opts.Safe {
		lintOpts.Include = &carve.Include{Root: opts.IncludeRoot, SourcePath: opts.SourcePath}
		rel, err := filepath.Rel(opts.IncludeRoot, opts.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("source path %q: %w", opts.SourcePath, err)
		}
		page = filepath.ToSlash(rel)
	}

	found, err := carve.LintWithOptions(body, lintOpts)
	if err != nil {
		return nil, err
	}
	findings := make([]Finding, 0, len(found))
	for _, f := range found {
		finding := Finding{File: f.File, Line: f.Line, Column: f.Column, Rule: f.Rule, Message: f.Message}
		if f.File == "" || f.File == page {
			finding.File = ""
			finding.Line, finding.Column = position(f.Line, f.Column)
		}
		for _, s := range f.IncludedBy {
			site := Site{File: s.File, Line: s.Line, Column: s.Column}
			if s.File == page {
				site.Line, site.Column = position(s.Line, s.Column)
			}
			finding.IncludedBy = append(finding.IncludedBy, site)
		}
		findings = append(findings, finding)
	}
	return findings, nil
}
