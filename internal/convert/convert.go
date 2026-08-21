// Package convert splits a Carve source file into its front matter and body,
// renders the Carve body to HTML via carve-go, and reassembles a Hugo-readable
// HTML content page (front matter preserved verbatim, body replaced with HTML).
//
// Hugo has no public API to register a new markup language for a custom file
// extension, so hugo-carve is a preprocessor: it produces `.html` content
// files that Hugo serves as passthrough page content while still reading the
// front matter for title and other params.
package convert

import (
	"fmt"
	"strings"

	carve "github.com/markup-carve/carve-go"
)

// fmDelims maps a front matter opening delimiter to its closing delimiter.
// TOML and YAML use a fenced form; JSON front matter is a brace-delimited
// object that Hugo also accepts.
type fmFormat struct {
	open  string
	close string
}

// Result is the outcome of converting one Carve document.
type Result struct {
	// FrontMatter is the raw front matter block (including its delimiters),
	// or the empty string when the source had none.
	FrontMatter string
	// BodyHTML is the rendered HTML of the Carve body.
	BodyHTML string
	// Output is the full file content to write: front matter (if any),
	// a blank line, then the rendered HTML.
	Output string
}

// Convert renders a Carve source document to a Hugo HTML content page.
//
// It detects and preserves a leading TOML (+++), YAML (---), or JSON front
// matter block, renders only the body through carve-go, and returns the
// reassembled output. It is pure: feeding the produced HTML body back in is a
// no-op for the front matter and never re-renders HTML as Carve, which keeps
// the CLI idempotent when run against already-converted trees.
func Convert(source string) (Result, error) {
	return ConvertWithOptions(source, Options{})
}

// Options configures a conversion. The zero value renders core Carve (no
// bundled extensions), matching Convert.
type Options struct {
	// Extensions enables the engine's bundled extensions - the diagram presets
	// (mermaid, plantuml, d2, graphviz, wavedrom, abc, vega-lite, chart) plus
	// details, spoiler, code-callouts, color and math. Diagram fences then
	// render as hydration elements (`<pre class="plantuml">`, ...) for a
	// client-side or build-step renderer; core Carve leaves them plain code.
	Extensions bool

	// Static produces self-contained HTML: interactive constructs are
	// flattened and diagrams/math degrade to their source. It implies
	// Extensions (that is what produces the constructs to flatten).
	Static bool

	// Symbols maps a shortcode name to the text `:NAME:` renders as. Carve
	// parses `:name:` in its core - no extension needed - but what a name
	// renders as is a render option, so with no map a shortcode renders as
	// its own source text. A name the map does not carry is left alone, and
	// the engine's word-boundary rule is untouched by a populated map:
	// `a:NAME:b` and `3:NAME:4` stay literal however the map is filled.
	//
	// The map is forwarded to the engine verbatim. It is deliberately not
	// re-validated here: carve-go refuses an entry that cannot reach the
	// engine intact (an empty name, a name containing "=", a NUL in either
	// half) with its own message, and a second validation layer here could
	// only disagree with it. Such an error surfaces through Convert rather
	// than being swallowed or silently dropping the entry.
	//
	// SECURITY: values are substituted RAW, exactly as written, and are NOT
	// escaped - that is what lets a symbol expand to markup such as an <img>
	// tag, and it is deliberate across every Carve engine. The map is trusted
	// processor configuration, on the same footing as the code calling this
	// package. Populate it from the site's own configuration and never from
	// page content, front matter, or anything else a document author
	// supplies.
	Symbols map[string]string
}

// ConvertWithOptions is Convert with explicit engine options.
func ConvertWithOptions(source string, opts Options) (Result, error) {
	fm, body := splitFrontMatter(source)

	carveOpts := carve.Options{Static: opts.Static, Symbols: opts.Symbols}
	if opts.Extensions || opts.Static {
		// carve-go enables the full bundle for any non-empty slice.
		carveOpts.Extensions = []string{"all"}
	}
	html, err := carve.ToHTMLOptions(body, carveOpts)
	if err != nil {
		return Result{}, fmt.Errorf("render carve body: %w", err)
	}
	html = strings.TrimRight(html, "\n")

	var b strings.Builder
	if fm != "" {
		b.WriteString(fm)
		// Ensure exactly one blank line between front matter and body so
		// Hugo's parser cleanly separates them.
		if !strings.HasSuffix(fm, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(html)
	b.WriteString("\n")

	return Result{FrontMatter: fm, BodyHTML: html, Output: b.String()}, nil
}

// splitFrontMatter separates a leading front matter block from the body.
// It returns the front matter block verbatim (with delimiters) and the
// remaining body. When no front matter is present, the front matter is empty
// and the whole input is the body.
func splitFrontMatter(source string) (frontMatter, body string) {
	// Hugo accepts a leading UTF-8 BOM; tolerate it without consuming it into
	// the front matter block.
	const bom = "\uFEFF"
	trimmed := strings.TrimPrefix(source, bom)
	lead := len(source) - len(trimmed)
	prefix := source[:lead]

	// Fenced front matter: TOML (+++) or YAML (---).
	for _, f := range []fmFormat{{"+++", "+++"}, {"---", "---"}} {
		if block, rest, ok := splitFenced(trimmed, f); ok {
			return prefix + block, rest
		}
	}

	// JSON front matter: a leading brace-balanced object.
	if block, rest, ok := splitJSON(trimmed); ok {
		return prefix + block, rest
	}

	return "", source
}

// splitFenced handles +++ / --- delimited front matter. The opening delimiter
// must be the very first line. Everything up to and including the closing
// delimiter line is the front matter block.
func splitFenced(s string, f fmFormat) (block, rest string, ok bool) {
	lines := strings.SplitAfter(s, "\n")
	if len(lines) == 0 {
		return "", "", false
	}
	if strings.TrimRight(lines[0], "\r\n") != f.open {
		return "", "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") == f.close {
			block = strings.Join(lines[:i+1], "")
			rest = strings.Join(lines[i+1:], "")
			return block, rest, true
		}
	}
	return "", "", false
}

// splitJSON handles brace-delimited JSON front matter at the start of the
// document. The very first byte must be '{' (Hugo only recognizes JSON front
// matter when it leads the file; leading whitespace before a brace block means
// it is body content, not front matter). The block ends at the matching
// closing brace (string-literal aware so braces in values do not confuse the
// scan).
func splitJSON(s string) (block, rest string, ok bool) {
	if len(s) == 0 || s[0] != '{' {
		return "", "", false
	}
	depth := 0
	inStr := false
	esc := false
	for j := 0; j < len(s); j++ {
		c := s[j]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:j+1], s[j+1:], true
			}
		}
	}
	return "", "", false
}
