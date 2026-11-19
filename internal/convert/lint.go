package convert

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	carve "github.com/markup-carve/carve-go/v2"
)

// Finding is one lint finding, positioned in the file it arose in.
type Finding struct {
	// File is the included file the finding arose in, relative to the include
	// root and slash-separated. Empty for the page's own findings.
	File    string
	Line    int
	Column  int
	Rule    string
	Message string
}

// Lint runs the engine's lint pass over a content file's Carve body.
//
// Front matter is split off first, the same way ConvertWithOptions does, since
// TOML and JSON front matter are not Carve. The engine reports UTF-8 byte
// ranges; they become 1-based lines and codepoint columns in the file.
//
// With Includes on, the pass also lints every included Carve file.
func Lint(source string, opts Options) ([]Finding, error) {
	_, body := splitFrontMatter(source)
	head := source[:len(source)-len(body)]

	lintOpts := carve.LintOptions{}
	if opts.Extensions {
		return nil, ErrExtensionsUnavailable
	}
	page := ""
	if opts.Includes {
		lintOpts.Include = &carve.Include{Root: opts.IncludeRoot, SourcePath: opts.SourcePath}
		rel, err := filepath.Rel(opts.IncludeRoot, opts.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("source path %q: %w", opts.SourcePath, err)
		}
		page = filepath.ToSlash(rel)
	}

	found, err := carve.Lint(context.Background(), body, lintOpts)
	if err != nil {
		return nil, err
	}
	findings := make([]Finding, 0, len(found))
	for _, d := range found {
		finding := Finding{File: d.File, Rule: d.Code, Message: d.Message}
		text, prefix := source, len(head)
		if d.File != "" && d.File != page {
			blob, err := os.ReadFile(filepath.Join(opts.IncludeRoot, filepath.FromSlash(d.File)))
			if err != nil {
				return nil, fmt.Errorf("read included file %q: %w", d.File, err)
			}
			text, prefix = string(blob), 0
		} else {
			finding.File = ""
		}
		if d.Range != nil {
			finding.Line, finding.Column = position(text, prefix+d.Range.StartOffset)
		}
		findings = append(findings, finding)
	}
	return findings, nil
}

// position turns a byte offset into a 1-based line and codepoint column.
func position(text string, offset int) (int, int) {
	offset = min(max(offset, 0), len(text))
	before := text[:offset]
	lineStart := strings.LastIndex(before, "\n") + 1
	return strings.Count(before, "\n") + 1, utf8.RuneCountInString(before[lineStart:]) + 1
}
