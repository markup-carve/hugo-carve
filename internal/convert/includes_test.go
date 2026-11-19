package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Include expansion, contained to a root the site names.
//
// The front matter is split off before the body reaches the engine, so these
// also pin that a directive still resolves from where the page actually lives
// rather than from the bytes handed over.

type site struct {
	root    string
	content string
}

func newSite(t *testing.T) site {
	t.Helper()
	root := t.TempDir()
	content := filepath.Join(root, "content")
	mkdir(t, filepath.Join(content, "sub"))
	mkdir(t, filepath.Join(root, "outside"))
	write(t, filepath.Join(content, "sub", "frag.crv"), "Fragment body here.\n")
	write(t, filepath.Join(root, "outside", "secret.crv"), "Uncontained body here.\n")
	return site{root: root, content: content}
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
}

func write(t *testing.T, path, body string) string {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
	return path
}

// page writes a Carve file under the content directory and returns the options
// that name it.
func (s site) page(t *testing.T, name, body string) (string, Options) {
	t.Helper()
	path := write(t, filepath.Join(s.content, name), body)
	return body, Options{Includes: true, IncludeRoot: s.content, SourcePath: path}
}

// --- the switch ----------------------------------------------------------

func TestIncludes_OffLeavesTheDirectiveLiteral(t *testing.T) {
	s := newSite(t)
	write(t, filepath.Join(s.content, "index.crv"), "{{ sub/frag.crv }}\n")
	res, err := Convert("{{ sub/frag.crv }}\n")
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(res.BodyHTML, "{{ sub/frag.crv }}") {
		t.Errorf("directive should stay literal, got %q", res.BodyHTML)
	}
	if strings.Contains(res.BodyHTML, "Fragment") {
		t.Errorf("nothing should have been expanded, got %q", res.BodyHTML)
	}
}

func TestIncludes_OnExpandsTheDirective(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "index.crv", "{{ sub/frag.crv }}\n")
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, "Fragment") {
		t.Errorf("expected the fragment, got %q", res.BodyHTML)
	}
}

// Expansion with nothing to contain it is refused. The refusal is carve-go's -
// an empty value is not an absolute path - and this package deliberately does
// not restate the rule, so what these pin is that it reaches the caller rather
// than being swallowed into a page with no includes in it.
func TestIncludes_WithoutARootIsRefused(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "index.crv", "{{ sub/frag.crv }}\n")
	opts.IncludeRoot = ""
	if _, err := ConvertWithOptions(body, opts); err == nil {
		t.Fatal("expected a refusal with no root")
	}
}

func TestIncludes_WithoutASourcePathIsRefused(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "index.crv", "{{ sub/frag.crv }}\n")
	opts.SourcePath = ""
	if _, err := ConvertWithOptions(body, opts); err == nil {
		t.Fatal("expected a refusal with no source path")
	}
}

// --- the root ------------------------------------------------------------

// A relative root is the engine's refusal to make, not this package's: section
// 19 forbids containment defaulting to the working directory, and absolutizing
// it anywhere on the way in is what would stop the refusal firing.
func TestIncludes_ARelativeRootIsRefused(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "index.crv", "{{ sub/frag.crv }}\n")
	// From here "content" resolves to the real root, so only the refusal fails it.
	t.Chdir(s.root)
	opts.IncludeRoot = "content"
	_, err := ConvertWithOptions(body, opts)
	if err == nil {
		t.Fatal("expected a relative root to be refused")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("expected the refusal to name the rule, got %v", err)
	}
}

func TestIncludes_AWiderRootReachesTheEngine(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "index.crv", "{{ ../outside/secret.crv }}\n")
	opts.IncludeRoot = s.root
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, "Uncontained") {
		t.Errorf("a root above the content directory should reach it, got %q", res.BodyHTML)
	}
}

// --- denials -------------------------------------------------------------

func TestIncludes_TraversalOutOfTheRootIsNotExpanded(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "index.crv", "{{ ../outside/secret.crv }}\n")
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if strings.Contains(res.BodyHTML, "Uncontained") {
		t.Errorf("containment should have refused this, got %q", res.BodyHTML)
	}
}

// C2-N-061 and C2-H-133: each cause and its render loss are reported.
func TestIncludes_ARefusalAndAMissingFileReportTheSameRule(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "index.crv", "{{ ../outside/secret.crv }}\n\n{{ nope.crv }}\n")
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	want := map[string]int{"include-outside-root": 1, "include-not-found": 1, "include-unresolved": 2}
	for _, w := range res.Warnings {
		want[w.Rule]--
	}
	for rule, count := range want {
		if count != 0 {
			t.Errorf("expected the 0.2 cause and loss reports, %s differs by %d: %+v", rule, count, res.Warnings)
		}
	}
}

func TestIncludes_ARefusalCarriesNoHostPath(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "index.crv", "{{ ../outside/secret.crv }}\n")
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("a refused include reported nothing")
	}
	for _, w := range res.Warnings {
		if strings.Contains(w.Message, s.root) {
			t.Errorf("a host path reached the message: %q", w.Message)
		}
	}
}

// --- dependencies --------------------------------------------------------

func TestIncludes_AReadTargetIsReportedAsAHostPath(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "index.crv", "{{ sub/frag.crv }}\n")
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	want := filepath.Join(s.content, "sub", "frag.crv")
	var found bool
	for _, d := range res.Dependencies {
		if d.Resolved && d.HostPath == want {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %q among the dependencies, got %+v", want, res.Dependencies)
	}
}

func TestIncludes_ARefusedTargetIsReportedUnresolved(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "index.crv", "{{ nope.crv }}\n")
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if len(res.Dependencies) == 0 {
		t.Fatal("a refused target should still be reported")
	}
	for _, d := range res.Dependencies {
		if d.Resolved {
			t.Errorf("nothing was read, got %+v", d)
		}
	}
}

func TestIncludes_NoDependenciesWhileExpansionIsOff(t *testing.T) {
	res, err := Convert("{{ sub/frag.crv }}\n")
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(res.Dependencies) != 0 || len(res.Warnings) != 0 {
		t.Errorf("expected nothing reported, got %+v / %+v", res.Dependencies, res.Warnings)
	}
}

// --- what the engine is asked to do --------------------------------------

// The page's own identity, not the root, is what a sibling resolves against.
func TestIncludes_APathResolvesAgainstThePageThatWroteIt(t *testing.T) {
	s := newSite(t)
	body, opts := s.page(t, "sub/page.crv", "{{ frag.crv }}\n")
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, "Fragment") {
		t.Errorf("a sibling should resolve, got %q", res.BodyHTML)
	}
}

func TestIncludes_ANestedPathResolvesAgainstTheIncludingFile(t *testing.T) {
	s := newSite(t)
	write(t, filepath.Join(s.content, "sub", "deep", "inner.crv"), "Innermost body here.\n")
	write(t, filepath.Join(s.content, "sub", "frag.crv"), "{{ deep/inner.crv }}\n")
	body, opts := s.page(t, "index.crv", "{{ sub/frag.crv }}\n")
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, "Innermost") {
		t.Errorf("expected the nested fragment, got %q", res.BodyHTML)
	}
}

func TestIncludes_ACycleDegradesInsteadOfHanging(t *testing.T) {
	s := newSite(t)
	// C2-N-002 expands block directives, never an inline mention.
	write(t, filepath.Join(s.content, "a.crv"), "Alpha\n\n{{ b.crv }}\n")
	write(t, filepath.Join(s.content, "b.crv"), "Beta\n\n{{ a.crv }}\n")
	body, opts := s.page(t, "index.crv", "{{ a.crv }}\n")
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, "Alpha") || !strings.Contains(res.BodyHTML, "Beta") {
		t.Errorf("expected the partial expansion, got %q", res.BodyHTML)
	}
	var cycle bool
	for _, w := range res.Warnings {
		if w.Rule == "include-cycle" {
			cycle = true
		}
	}
	if !cycle {
		t.Errorf("expected include-cycle, got %+v", res.Warnings)
	}
}

// The body reaching the engine has had its front matter split off, while the
// file on disk still carries it. The mount serves the body for the document's
// own path, so both halves have to hold at once.
func TestIncludes_FrontMatterIsSplitOffAndTheDirectiveStillResolves(t *testing.T) {
	s := newSite(t)
	source := "+++\ntitle = \"T\"\n+++\n\n{{ sub/frag.crv }}\n"
	path := write(t, filepath.Join(s.content, "index.crv"), source)
	res, err := ConvertWithOptions(source, Options{
		Includes: true, IncludeRoot: s.content, SourcePath: path,
	})
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.FrontMatter, "title") {
		t.Errorf("front matter should be preserved, got %q", res.FrontMatter)
	}
	if strings.Contains(res.BodyHTML, "title") {
		t.Errorf("front matter should not have been rendered, got %q", res.BodyHTML)
	}
	if !strings.Contains(res.BodyHTML, "Fragment") {
		t.Errorf("expected the fragment, got %q", res.BodyHTML)
	}
}

// Every render option the page is parsed with reaches an included child too.
func TestIncludes_TheSymbolMapReachesAnIncludedChild(t *testing.T) {
	s := newSite(t)
	write(t, filepath.Join(s.content, "sub", "frag.crv"), "Fragment with :crv:\n")
	body, opts := s.page(t, "index.crv", "{{ sub/frag.crv }}\n")
	opts.Symbols = map[string]string{"crv": "CARVE"}
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, "CARVE") {
		t.Errorf("the symbol map should reach the child, got %q", res.BodyHTML)
	}
}

func TestIncludes_TheProfileGovernsAnIncludedChild(t *testing.T) {
	s := newSite(t)
	write(t, filepath.Join(s.content, "sub", "frag.crv"), "a [home](https://example.com/x) b\n")
	body, opts := s.page(t, "index.crv", "{{ sub/frag.crv }}\n")
	opts.Profile = "minimal"
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if strings.Contains(res.BodyHTML, "<a ") {
		t.Errorf("the profile should have filtered the child's link, got %q", res.BodyHTML)
	}
	// C2-N-077 degrades the include node under the minimal profile.
	if strings.Contains(res.BodyHTML, "home") || !strings.Contains(res.BodyHTML, "{{ sub/frag.crv }}") {
		t.Errorf("the include should have degraded under minimal, got %q", res.BodyHTML)
	}
}

// C2-N-001: the engine expands a named file when includes are enabled.
func TestIncludes_TheEngineExpandsTheFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "part.crv"), []byte("From the engine.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ConvertWithOptions("{{ part.crv }}\n", Options{
		Includes:    true,
		IncludeRoot: root,
		SourcePath:  filepath.Join(root, "page.crv"),
	})
	if err != nil || !strings.Contains(res.BodyHTML, "From the engine.") {
		t.Fatalf("expected include expansion, got %q, %v", res.BodyHTML, err)
	}
}
