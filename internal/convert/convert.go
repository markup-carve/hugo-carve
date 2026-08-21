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

	// Safe escapes raw HTML instead of emitting it: a `=html` fenced block
	// and a `{=html}` inline span both render as visible, escaped text. It is
	// off by default, which is the behavior this package has always had.
	//
	// It covers raw passthrough and nothing else, because nothing else needs
	// covering: Carve's normative hardening is always on, so a dangerous URL
	// scheme is blanked, an event-handler attribute is dropped and the bidi
	// override characters behind Trojan Source are removed whether Safe is set
	// or not. Raw passthrough is the deliberate exception - a `=html` block
	// renders verbatim by design - so it is the one thing a document you did
	// not author has to be able to switch off.
	//
	// Safe is a statement about the DOCUMENT, not about this package's
	// configuration. In particular it does NOT constrain Symbols: a symbol
	// value is substituted raw with Safe on exactly as it is with Safe off,
	// because the map is processor configuration rather than page content.
	// See the security note on Symbols.
	//
	// Set it for any tree whose pages can come from outside the site's own
	// authors - a docs site taking contributions, or any build where a page
	// can arrive in a pull request.
	Safe bool

	// Profile restricts what a document may contain and how large it may be.
	// It maps to the engine's --profile flag and takes one of the engine's own
	// names: "full" (the default behavior), "article", "comment" or "minimal".
	// The empty string leaves the option off entirely, which is what this
	// package has always done.
	//
	// The name is forwarded verbatim and is NOT re-validated here: an unknown
	// or wrongly-cased name comes back as the engine's own message
	// (`carve: unknown profile: nope (expected full|article|comment|minimal)`),
	// and a second list here could only disagree with it.
	//
	// CAPS. "comment" and "minimal" also cap the input, and the cap applies to
	// the BODY handed to the engine - front matter is split off first and does
	// not count toward it. Measured against the pinned engine: "comment"
	// accepts 100000 bytes, "minimal" accepts 10000 bytes, and "full" and
	// "article" have no cap under 4 MB. The count is bytes, not runes: 5001
	// two-byte characters is 10001 bytes and is over the "minimal" cap.
	//
	// An over-cap body is a hard ERROR from ConvertWithOptions, never a page.
	// See the profileMaxBytes comment for why this package refuses it itself
	// rather than waiting for the engine to.
	Profile string

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

	carveOpts := carve.Options{Static: opts.Static, Safe: opts.Safe, Profile: opts.Profile, Symbols: opts.Symbols}
	if opts.Extensions || opts.Static {
		// carve-go enables the full bundle for any non-empty slice.
		carveOpts.Extensions = []string{"all"}
	}
	html, err := carve.ToHTMLOptions(body, carveOpts)
	if err != nil {
		return Result{}, fmt.Errorf("render carve body: %w", err)
	}
	if err := checkProfileCap(opts.Profile, body, html); err != nil {
		return Result{}, err
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

// profileMaxBytes is the input cap each named profile enforces, in bytes of
// the BODY handed to the engine. "full" and "article" are absent because they
// cap nothing. The numbers are the engine's, measured against the pinned
// carve-go (hugo-carve#20).
//
// A second copy of someone else's number is normally the wrong shape, and this
// package has twice declined to re-validate what the engine already checks -
// a symbol name, a profile name. Both of those come back as an ERROR, so
// restating them here could only produce a competing message for the same
// refusal. An over-cap body does not: the engine embedded in the pinned
// carve-go answers it with an empty render, exit 0 and an empty stderr, so a
// misconfigured build publishes a BLANK PAGE and nothing anywhere says why
// (markup-carve/carve-rs#1190). For a preprocessor whose output Hugo serves,
// that failure reaches readers and no one upstream sees it.
//
// So the guard fires on the LOSS, not on the length. It needs BOTH an
// over-cap body and an empty render, which is what keeps a stale number from
// rotting into a wrong refusal in either direction:
//
//   - the engine raises a cap: an over-cap body still renders, the render is
//     not empty, and nothing is refused.
//   - the engine lowers a cap: the engine refuses first, and its own error
//     surfaces through ConvertWithOptions above.
//   - the engine learns to refuse (carve-rs#1194 is that fix, and carve-go's
//     embedded wasm predates it): the engine's error surfaces above and this
//     guard stops firing on its own, with no change here.
//
// It also cannot mistake a body that legitimately renders to nothing - a page
// whose body is only a `%% ... %%` comment renders empty with or without a
// profile - because such a body is not over the cap.
var profileMaxBytes = map[string]int{
	"comment": 100_000,
	"minimal": 10_000,
}

// checkProfileCap turns a silently discarded over-cap body into an error
// naming the cap and the actual size. It returns nil for every other outcome,
// including every profile that caps nothing.
func checkProfileCap(profile, body, html string) error {
	max, capped := profileMaxBytes[profile]
	if !capped || len(body) <= max {
		return nil
	}
	if strings.TrimSpace(html) != "" {
		return nil
	}
	return fmt.Errorf(
		"profile %q discarded the whole body: it is %d bytes and the %q profile caps input at %d bytes "+
			"(front matter is not counted). Split the page, or render it with the \"article\" or \"full\" profile",
		profile, len(body), profile, max)
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
