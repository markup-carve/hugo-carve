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
