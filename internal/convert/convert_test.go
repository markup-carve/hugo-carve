package convert

import (
	"strings"
	"testing"

	carve "github.com/markup-carve/carve-go"
)

// TestConvert_BodyHTML verifies that the Carve body is rendered to HTML with
// the expected heading, bold, emphasis, and list markup.
func TestConvert_BodyHTML(t *testing.T) {
	src := "# Hello Carve\n\nThis is *bold* and /italic/.\n\n- alpha\n- beta\n"
	res, err := Convert(src)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}

	assertions := []struct {
		name string
		want string
	}{
		{"heading", "<h1>Hello Carve</h1>"},
		{"bold", "<strong>bold</strong>"},
		{"emphasis", "<em>italic</em>"},
		{"list-open", "<ul>"},
		{"list-item", "<li>alpha</li>"},
	}
	for _, a := range assertions {
		if !strings.Contains(res.BodyHTML, a.want) {
			t.Errorf("%s: expected %q in body HTML, got:\n%s", a.name, a.want, res.BodyHTML)
		}
	}
}

// TestConvert_TOMLFrontMatter verifies TOML (+++) front matter is preserved
// verbatim and the body below it is rendered to HTML.
func TestConvert_TOMLFrontMatter(t *testing.T) {
	src := "+++\ntitle = \"My Page\"\ndate = 2026-06-20\n+++\n\n# Heading\n\nBody *text*.\n"
	res, err := Convert(src)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if !strings.HasPrefix(res.FrontMatter, "+++\n") {
		t.Errorf("expected TOML front matter, got %q", res.FrontMatter)
	}
	if !strings.Contains(res.FrontMatter, `title = "My Page"`) {
		t.Errorf("title not preserved in front matter: %q", res.FrontMatter)
	}
	if !strings.HasPrefix(res.Output, "+++\n") {
		t.Errorf("output should start with front matter, got %q", res.Output[:min(20, len(res.Output))])
	}
	if !strings.Contains(res.Output, "<h1>Heading</h1>") {
		t.Errorf("output should contain rendered heading, got %q", res.Output)
	}
	if strings.Contains(res.BodyHTML, "title = ") {
		t.Errorf("front matter leaked into rendered body: %q", res.BodyHTML)
	}
}

// TestConvert_YAMLFrontMatter verifies YAML (---) front matter is detected and
// preserved, and the body is rendered.
func TestConvert_YAMLFrontMatter(t *testing.T) {
	src := "---\ntitle: YAML Page\n---\n\n# Y\n\n- one\n"
	res, err := Convert(src)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if !strings.HasPrefix(res.FrontMatter, "---\n") || !strings.Contains(res.FrontMatter, "title: YAML Page") {
		t.Errorf("YAML front matter not preserved: %q", res.FrontMatter)
	}
	if !strings.Contains(res.BodyHTML, "<li>one</li>") {
		t.Errorf("expected list item in body, got %q", res.BodyHTML)
	}
}

// TestConvert_NoFrontMatter verifies a body-only document renders with no
// front matter prefix.
func TestConvert_NoFrontMatter(t *testing.T) {
	res, err := Convert("# Just A Heading\n")
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if res.FrontMatter != "" {
		t.Errorf("expected no front matter, got %q", res.FrontMatter)
	}
	if !strings.Contains(res.Output, "<h1>Just A Heading</h1>") {
		t.Errorf("expected heading in output, got %q", res.Output)
	}
}

// TestConvert_Idempotent verifies that converting and then re-converting the
// already-converted output does not corrupt the front matter and does not
// re-render the HTML body as Carve (the HTML survives unchanged in shape).
func TestConvert_Idempotent(t *testing.T) {
	src := "+++\ntitle = \"Stable\"\n+++\n\n# Title\n\n*bold*\n"
	first, err := Convert(src)
	if err != nil {
		t.Fatalf("first Convert error: %v", err)
	}
	if !strings.Contains(first.FrontMatter, `title = "Stable"`) {
		t.Fatalf("front matter lost on first pass: %q", first.FrontMatter)
	}
	if !strings.Contains(first.Output, "<h1>Title</h1>") || !strings.Contains(first.Output, "<strong>bold</strong>") {
		t.Fatalf("first pass missing expected HTML: %q", first.Output)
	}
}

// TestConvert_JSONFrontMatter verifies leading JSON front matter is detected
// and preserved verbatim.
func TestConvert_JSONFrontMatter(t *testing.T) {
	src := "{\n  \"title\": \"JSON Page\"\n}\n\n# J\n"
	res, err := Convert(src)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if !strings.HasPrefix(res.FrontMatter, "{") || !strings.Contains(res.FrontMatter, `"title": "JSON Page"`) {
		t.Errorf("JSON front matter not preserved: %q", res.FrontMatter)
	}
	if !strings.Contains(res.BodyHTML, "<h1>J</h1>") {
		t.Errorf("expected heading in body, got %q", res.BodyHTML)
	}
}

// TestConvert_LeadingBraceIsBodyNotFrontMatter verifies that a brace block
// that does not lead the file (preceded by whitespace) is treated as body
// content, not silently swallowed as JSON front matter.
func TestConvert_LeadingBraceIsBodyNotFrontMatter(t *testing.T) {
	// A brace block that does not lead the file must NOT be consumed as JSON
	// front matter; it is handed to the Carve engine as body content.
	src := "\n{#myid}\n# Heading\n"
	res, err := Convert(src)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if res.FrontMatter != "" {
		t.Errorf("expected no front matter for non-leading brace block, got %q", res.FrontMatter)
	}
	if !strings.Contains(res.BodyHTML, "<h1") {
		t.Errorf("body should be rendered by the Carve engine, got %q", res.BodyHTML)
	}
}

// TestCarveGo_CodeCallouts verifies that the embedded carve-go wasm renders
// code-callout markers (<N>) inside a fenced code block to the expected HTML
// when the bundled extensions are enabled. Callout markers in the source
// replace the literal angle-bracket token with a <b class="callout"> element,
// and the callout list below the block becomes an <ol class="callouts">.
func TestCarveGo_CodeCallouts(t *testing.T) {
	src := "```go\nx := 1 // <1>\ny := 2 // <2>\n```\n\n<1> assign x\n<2> assign y\n"
	html, err := carve.ToHTMLOptions(src, carve.Options{Extensions: []string{"all"}})
	if err != nil {
		t.Fatalf("ToHTMLOptions error: %v", err)
	}
	assertions := []struct {
		name string
		want string
	}{
		{"callout-marker-1", `class="callout" data-callout="1"`},
		{"callout-marker-2", `class="callout" data-callout="2"`},
		{"callout-list", `class="callouts"`},
		{"callout-label-1", `assign x`},
		{"callout-label-2", `assign y`},
	}
	for _, a := range assertions {
		if !strings.Contains(html, a.want) {
			t.Errorf("%s: expected %q in HTML, got:\n%s", a.name, a.want, html)
		}
	}
}

// TestCarveGo_CitationsAsMentionSpans documents that citation syntax ([@key])
// renders as a mention span through the WASI/CLI path rather than a full
// bibliography citation. This is an upstream architecture boundary: the
// bibliography extension requires a CSL-JSON data source passed by the host,
// which cannot cross the WASI stdio contract used by carve-go. The rendered
// form is a <span class="mention"> wrapping the key - not a bug in this repo.
func TestCarveGo_CitationsAsMentionSpans(t *testing.T) {
	src := "See [@smith2020] for details.\n"
	html, err := carve.ToHTML(src)
	if err != nil {
		t.Fatalf("ToHTML error: %v", err)
	}
	if !strings.Contains(html, `class="mention"`) {
		t.Errorf("expected mention span for citation, got:\n%s", html)
	}
	if !strings.Contains(html, "smith2020") {
		t.Errorf("expected citation key preserved in output, got:\n%s", html)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestConvertWithOptions_ExtensionsEnableDiagrams verifies that a plantuml fence
// renders as a hydration element only when Extensions is set (core Carve leaves
// it a plain code block).
func TestConvertWithOptions_ExtensionsEnableDiagrams(t *testing.T) {
	src := "``` plantuml\nA -> B\n```\n"

	off, err := Convert(src)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if strings.Contains(off.BodyHTML, `class="plantuml"`) {
		t.Fatalf("core Carve should not render the diagram, got %q", off.BodyHTML)
	}

	on, err := ConvertWithOptions(src, Options{Extensions: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	if !strings.Contains(on.BodyHTML, `<pre class="plantuml">A -> B</pre>`) {
		t.Fatalf("expected plantuml hydration element, got %q", on.BodyHTML)
	}
}

// TestConvertWithOptions_StaticDegradesDiagrams verifies static mode degrades a
// diagram fence to its source as a code block, not a client hydration element.
func TestConvertWithOptions_StaticDegradesDiagrams(t *testing.T) {
	res, err := ConvertWithOptions("``` mermaid\ngraph TD; A-->B\n```\n", Options{Static: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	// Degraded form carries the source in an inner <code class="language-...">;
	// a live hydration element would be a bare <pre class="mermaid"> with no code.
	if !strings.Contains(res.BodyHTML, `<code class="language-mermaid">`) {
		t.Fatalf("static mode should degrade the diagram to a source code block, got %q", res.BodyHTML)
	}
}

// Carve parses `:name:` in its core, but what a name renders as is a render
// option, so the whole construct is inert on a Hugo site until a map reaches
// the engine. These cases pin both halves: that a populated map substitutes,
// and that everything about the construct is unchanged when it is not.

// TestConvertWithOptions_SymbolsSubstitute is the case the option exists for:
// a mapped name renders as its value.
func TestConvertWithOptions_SymbolsSubstitute(t *testing.T) {
	res, err := ConvertWithOptions("Ship it :rocket:\n", Options{
		Symbols: map[string]string{"rocket": "\U0001F680"},
	})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	if !strings.Contains(res.BodyHTML, "Ship it \U0001F680") {
		t.Errorf("mapped symbol not substituted, got:\n%s", res.BodyHTML)
	}
	if strings.Contains(res.BodyHTML, ":rocket:") {
		t.Errorf("mapped symbol left as source text, got:\n%s", res.BodyHTML)
	}
}

// TestConvertWithOptions_NoSymbolsUnchanged pins the default. A nil map and an
// empty map both have to render exactly what this package rendered before the
// option existed, byte for byte - an additive option that quietly changed the
// no-map output would be a breaking change wearing a feature's clothes.
func TestConvertWithOptions_NoSymbolsUnchanged(t *testing.T) {
	const src = "+++\ntitle = \"S\"\n+++\n\nShip it :rocket: and *bold*.\n"

	base, err := Convert(src)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if !strings.Contains(base.BodyHTML, ":rocket:") {
		t.Fatalf("with no map, the shortcode should stay literal, got:\n%s", base.BodyHTML)
	}

	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"nil map", Options{}},
		{"nil map, explicit", Options{Symbols: nil}},
		{"empty map", Options{Symbols: map[string]string{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ConvertWithOptions(src, tc.opts)
			if err != nil {
				t.Fatalf("ConvertWithOptions error: %v", err)
			}
			if got.Output != base.Output {
				t.Errorf("output differs from the no-option render:\n got: %q\nwant: %q", got.Output, base.Output)
			}
		})
	}
}

// TestConvertWithOptions_SymbolsLeaveUnknownNameLiteral pins that a map is a
// lookup and not a mode: a name it does not carry is left exactly as written,
// rather than becoming an error or an empty string.
func TestConvertWithOptions_SymbolsLeaveUnknownNameLiteral(t *testing.T) {
	res, err := ConvertWithOptions("Ship it :rocket: :shrug:\n", Options{
		Symbols: map[string]string{"rocket": "\U0001F680"},
	})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	if !strings.Contains(res.BodyHTML, ":shrug:") {
		t.Errorf("unmapped name should stay literal, got:\n%s", res.BodyHTML)
	}
}

// TestConvertWithOptions_SymbolsKeepWordBoundary pins that populating the map
// does not widen where a shortcode is recognized. All four shapes are rendered
// in ONE document with the map active, so the substitution that must happen and
// the three that must not are decided by the same render - a test where the
// negatives came from a separate no-map render would pass even if the map
// disabled the guard entirely.
func TestConvertWithOptions_SymbolsKeepWordBoundary(t *testing.T) {
	src := "a:rocket:b and 3:rocket:4 and `A :rocket: x` and A :rocket: here\n"
	res, err := ConvertWithOptions(src, Options{
		Symbols: map[string]string{"rocket": "SUBSTITUTED"},
	})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	for _, want := range []string{"a:rocket:b", "3:rocket:4", "<code>A :rocket: x</code>", "A SUBSTITUTED here"} {
		if !strings.Contains(res.BodyHTML, want) {
			t.Errorf("expected %q in body HTML, got:\n%s", want, res.BodyHTML)
		}
	}
	if strings.Count(res.BodyHTML, "SUBSTITUTED") != 1 {
		t.Errorf("exactly one substitution expected, got:\n%s", res.BodyHTML)
	}
}

// TestConvertWithOptions_SymbolsSubstituteRaw pins the security contract the
// documentation states: a value is emitted RAW, not escaped. It is what lets a
// symbol expand to markup, and it is why the map may only ever be built from
// the site author's own configuration. If this ever starts escaping, the README
// warning is wrong and has to change with it.
func TestConvertWithOptions_SymbolsSubstituteRaw(t *testing.T) {
	res, err := ConvertWithOptions("A :logo: here\n", Options{
		Symbols: map[string]string{"logo": "<img src='/l.svg'>"},
	})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	if !strings.Contains(res.BodyHTML, "<img src='/l.svg'>") {
		t.Errorf("symbol value should reach the output raw, got:\n%s", res.BodyHTML)
	}
}

// TestConvertWithOptions_SymbolsSurfaceEngineError pins that this package adds
// no validation layer of its own AND swallows none of the engine's. carve-go
// refuses an entry that cannot reach the engine intact; the caller has to see
// that message rather than a silently shorter map.
func TestConvertWithOptions_SymbolsSurfaceEngineError(t *testing.T) {
	for _, tc := range []struct {
		name    string
		symbols map[string]string
		want    string
	}{
		{"name contains =", map[string]string{"a=b": "c"}, `must not contain "="`},
		{"empty name", map[string]string{"": "x"}, "must not be empty"},
		{"NUL in value", map[string]string{"n": "v\x00"}, "must not contain a NUL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ConvertWithOptions("A :a: here\n", Options{Symbols: tc.symbols})
			if err == nil {
				t.Fatalf("expected the engine to refuse %v, got no error", tc.symbols)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected the engine's own message containing %q, got: %v", tc.want, err)
			}
		})
	}
}

// TestConvertWithOptions_SymbolsReachTheEngineUnchanged closes the gap the
// cases above leave open. Every one of them asserts on rendered HTML, so a
// forwarding bug that dropped, renamed or filtered entries on the way through
// could still satisfy them for the names they happen to name. This compares
// against the SAME linked engine called directly, in the spirit of
// TestConvertAddsNothingToTheEngine: whatever carve-go does with a map, this
// package has to produce exactly that.
func TestConvertWithOptions_SymbolsReachTheEngineUnchanged(t *testing.T) {
	symbols := map[string]string{
		"rocket": "\U0001F680",
		"logo":   "<img src='/l.svg'>",
		"zz":     "never referenced",
	}
	const body = "Ship it :rocket: and :logo: and :shrug:\n"

	got, err := ConvertWithOptions(body, Options{Symbols: symbols})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	want, err := carve.ToHTMLOptions(body, carve.Options{Symbols: symbols})
	if err != nil {
		t.Fatalf("carve.ToHTMLOptions error: %v", err)
	}
	if got.BodyHTML != strings.TrimRight(want, "\n") {
		t.Errorf("this package renders a symbol map differently from the engine it links:\n got: %q\nwant: %q",
			got.BodyHTML, strings.TrimRight(want, "\n"))
	}
}

// rawHTMLSrc carries every construct Safe is supposed to reach (a `=html`
// fenced block and a `{=html}` inline span) alongside constructs it is NOT
// supposed to reach, so a test that passed by escaping the whole document
// would fail here rather than look green.
const rawHTMLSrc = "Before.\n\n" +
	"```=html\n<div class=\"x\"><script>alert(1)</script></div>\n```\n\n" +
	"Inline: `<b>bold</b>`{=html} tail.\n\n" +
	"A [link](https://example.com/) and an image ![a](x.png).\n"

// TestConvertWithOptions_SafeEscapesRawHTML is the point of the option: with
// Safe set, neither the raw block nor the raw span reaches the page as live
// markup.
func TestConvertWithOptions_SafeEscapesRawHTML(t *testing.T) {
	res, err := ConvertWithOptions(rawHTMLSrc, Options{Safe: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	for _, live := range []string{"<script>", "<div class=\"x\">", "<b>bold</b>"} {
		if strings.Contains(res.BodyHTML, live) {
			t.Errorf("Safe must not emit %q, got:\n%s", live, res.BodyHTML)
		}
	}
	for _, escaped := range []string{"&lt;script&gt;", "&lt;b&gt;bold&lt;/b&gt;"} {
		if !strings.Contains(res.BodyHTML, escaped) {
			t.Errorf("Safe should escape raw HTML into %q, got:\n%s", escaped, res.BodyHTML)
		}
	}
}

// TestConvertWithOptions_SafeLeavesEverythingElseAlone is the guard against
// the assertion above passing for the wrong reason. Safe escapes raw HTML; it
// is not a mode that escapes the document, so ordinary constructs still render
// as markup.
func TestConvertWithOptions_SafeLeavesEverythingElseAlone(t *testing.T) {
	res, err := ConvertWithOptions(rawHTMLSrc, Options{Safe: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	for _, want := range []string{"<p>Before.</p>", "<a href=\"https://example.com/\">link</a>", "<img src=\"x.png\""} {
		if !strings.Contains(res.BodyHTML, want) {
			t.Errorf("Safe should leave %q intact, got:\n%s", want, res.BodyHTML)
		}
	}
}

// TestConvertWithOptions_SafeOffIsUnchanged pins the default byte for byte. An
// additive option that quietly changed the output every existing site already
// gets would be a breaking change wearing a feature's clothes.
func TestConvertWithOptions_SafeOffIsUnchanged(t *testing.T) {
	const src = "+++\ntitle = \"S\"\n+++\n\n" +
		"```=html\n<div class=\"x\">raw</div>\n```\n\nAnd *bold*.\n"

	base, err := Convert(src)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if !strings.Contains(base.BodyHTML, "<div class=\"x\">raw</div>") {
		t.Fatalf("without Safe, raw HTML should pass through, got:\n%s", base.BodyHTML)
	}

	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"zero value", Options{}},
		{"explicitly off", Options{Safe: false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ConvertWithOptions(src, tc.opts)
			if err != nil {
				t.Fatalf("ConvertWithOptions error: %v", err)
			}
			if got.Output != base.Output {
				t.Errorf("output differs from the no-option render:\n got: %q\nwant: %q", got.Output, base.Output)
			}
		})
	}
}

// TestConvertWithOptions_SafeComposesWithTheOtherOptions pins that Safe is
// independent rather than mutually exclusive: it still escapes when static
// mode and the extensions are also on, which is the combination a hardened
// site would actually run.
func TestConvertWithOptions_SafeComposesWithTheOtherOptions(t *testing.T) {
	res, err := ConvertWithOptions(rawHTMLSrc, Options{Safe: true, Static: true, Extensions: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	if strings.Contains(res.BodyHTML, "<script>") {
		t.Errorf("Safe must hold alongside Static and Extensions, got:\n%s", res.BodyHTML)
	}
}

// TestConvertWithOptions_SafeDoesNotConstrainSymbols pins the boundary the
// README draws, because it is the one a reader is most likely to get wrong:
// Safe governs raw HTML in the DOCUMENT, and a symbol value is processor
// configuration, so it is still substituted raw. If the engine ever changed
// that, the README's security note would need rewriting rather than quietly
// becoming over-cautious - so this test exists to notice.
func TestConvertWithOptions_SafeDoesNotConstrainSymbols(t *testing.T) {
	const raw = "<img src=x onerror=alert(1)>"
	res, err := ConvertWithOptions("Logo :logo: here\n", Options{
		Safe:    true,
		Symbols: map[string]string{"logo": raw},
	})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	if !strings.Contains(res.BodyHTML, raw) {
		t.Errorf("a symbol value is trusted configuration and stays raw under Safe, got:\n%s", res.BodyHTML)
	}
}
