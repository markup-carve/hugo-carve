package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const unattached = "para\n\n{#id .cls}\n"

// runCapture runs the command and returns what it wrote to stderr and its error.
func runCapture(t *testing.T, args ...string) (string, error) {
	t.Helper()
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open devnull: %v", err)
	}
	defer devnull.Close()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("create stderr: %v", err)
	}
	defer stderr.Close()
	runErr := run(args, devnull, stderr)
	logged, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(logged), runErr
}

func lintSite(t *testing.T, page string) string {
	t.Helper()
	content := filepath.Join(t.TempDir(), "content")
	writeFile(t, filepath.Join(content, "page.crv"), page)
	return content
}

func TestLint_OffByDefault(t *testing.T) {
	content := lintSite(t, unattached)
	logged, err := runCapture(t, "--content", content, "--quiet")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if strings.Contains(logged, "unattached-block-attribute") {
		t.Errorf("lint ran without -lint: %q", logged)
	}
}

func TestLint_ReportsAFindingAndStillConverts(t *testing.T) {
	content := lintSite(t, unattached)
	logged, err := runCapture(t, "--content", content, "--quiet", "--lint")
	if err != nil {
		t.Fatalf("-lint must not fail the run: %v", err)
	}
	want := filepath.Join(content, "page.crv") + ":3:1: unattached-block-attribute: "
	if !strings.Contains(logged, want) {
		t.Errorf("expected %q in %q", want, logged)
	}
	if _, err := os.Stat(filepath.Join(content, "page.html")); err != nil {
		t.Errorf("page was not converted: %v", err)
	}
}

func TestLint_StrictFailsOnAFinding(t *testing.T) {
	content := lintSite(t, unattached)
	_, err := runCapture(t, "--content", content, "--quiet", "--lint-strict")
	if err == nil || !strings.Contains(err.Error(), "1 lint finding") {
		t.Fatalf("expected a lint failure, got %v", err)
	}
}

func TestLint_StrictPassesACleanPage(t *testing.T) {
	content := lintSite(t, "# Clean\n\nText.\n")
	logged, err := runCapture(t, "--content", content, "--quiet", "--lint-strict")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if logged != "" {
		t.Errorf("expected no output, got %q", logged)
	}
}

// TOML front matter is split off before the engine sees the body, so the
// finding's line has to be shifted back to the line in the file.
func TestLint_LineCountsTheFrontMatter(t *testing.T) {
	content := lintSite(t, "+++\ntitle = \"x\"\n+++\n"+unattached)
	logged, _ := runCapture(t, "--content", content, "--quiet", "--lint")
	if !strings.Contains(logged, "page.crv:6:1: unattached-block-attribute") {
		t.Errorf("expected line 6, got %q", logged)
	}
}

func TestLint_NotRunByClean(t *testing.T) {
	content := lintSite(t, unattached)
	logged, err := runCapture(t, "--content", content, "--quiet", "--clean", "--lint-strict")
	if err != nil {
		t.Fatalf("clean with -lint-strict: %v", err)
	}
	if logged != "" {
		t.Errorf("clean linted: %q", logged)
	}
}

// The bundle changes which findings the pass reports: with it on, the engine
// does not report a fragment link as broken. That difference is what shows
// -extensions reaching the lint pass.
func TestLint_ExtensionsReachTheLintPass(t *testing.T) {
	content := lintSite(t, "[x](#nope)\n")
	logged, _ := runCapture(t, "--content", content, "--quiet", "--lint")
	if !strings.Contains(logged, "broken-fragment-link") {
		t.Fatalf("core pass should report the link, got %q", logged)
	}
	logged, _ = runCapture(t, "--content", content, "--quiet", "--lint", "--extensions")
	if strings.Contains(logged, "broken-fragment-link") {
		t.Errorf("bundle pass should not report the link, got %q", logged)
	}
}

// JSON front matter may end on the line the body starts on.
func TestLint_ColumnCountsInlineJSONFrontMatter(t *testing.T) {
	content := lintSite(t, "{\"title\":\"x\"}[x](#nope)\n")
	logged, _ := runCapture(t, "--content", content, "--quiet", "--lint")
	if !strings.Contains(logged, "page.crv:1:14: broken-fragment-link") {
		t.Errorf("expected column 14, got %q", logged)
	}
}

func includeLintSite(t *testing.T) string {
	t.Helper()
	content := filepath.Join(t.TempDir(), "content")
	writeFile(t, filepath.Join(content, "parts", "frag.txt"), unattached)
	writeFile(t, filepath.Join(content, "page.crv"), "+++\ntitle = \"x\"\n+++\nIntro.\n\n{{ parts/frag.txt }}\n\n{{ missing.txt }}\n")
	return content
}

// With -includes the pass lints the included file too. The include site is
// in the page, so it counts the front matter like any page position.
func TestLint_FollowsIncludes(t *testing.T) {
	content := includeLintSite(t)
	logged, err := runCapture(t, "--content", content, "--quiet", "--includes", "--lint-strict")
	if err == nil {
		t.Fatal("expected -lint-strict to fail on the included finding")
	}
	want := "page.crv: parts/frag.txt:3:1: unattached-block-attribute: "
	if !strings.Contains(logged, want) || !strings.Contains(logged, "(included at page.crv:6:1)") {
		t.Errorf("expected %q with its site, got %q", want, logged)
	}
	if n := strings.Count(logged, "include-unresolved"); n != 1 {
		t.Errorf("an unresolved include should be reported once, got %d in %q", n, logged)
	}
}

func TestLint_WithoutIncludesIgnoresIncludedFiles(t *testing.T) {
	content := includeLintSite(t)
	logged, err := runCapture(t, "--content", content, "--quiet", "--lint-strict")
	if err != nil {
		t.Fatalf("run: %v (%q)", err, logged)
	}
	if strings.Contains(logged, "frag.txt") {
		t.Errorf("lint followed an include with -includes off: %q", logged)
	}
}

// -safe reads no files: includes stay literal and the lint pass opens none.
func TestSafe_LeavesIncludesLiteralAndSaysSo(t *testing.T) {
	content := includeLintSite(t)
	logged, err := runCapture(t, "--content", content, "--quiet", "--includes", "--safe", "--lint-strict")
	if err != nil {
		t.Fatalf("run: %v (%q)", err, logged)
	}
	if !strings.Contains(logged, "-safe reads no files") {
		t.Errorf("expected the -safe note, got %q", logged)
	}
	out, readErr := os.ReadFile(filepath.Join(content, "page.html"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(out), "{{ parts/frag.txt }}") {
		t.Errorf("expected the directive to stay literal under -safe, got %q", out)
	}
}

// carve-go v0.1.8 carries carve-rs 0.1.11, which selects `@region:name`.
func TestIncludes_SelectARegion(t *testing.T) {
	content := filepath.Join(t.TempDir(), "content")
	writeFile(t, filepath.Join(content, "snippet.crv"), "Before.\n\n%% #region keep\nKept line.\n%% #endregion keep\n\nAfter.\n")
	writeFile(t, filepath.Join(content, "page.crv"), "{{ snippet.crv @region:keep }}\n")
	runCLI(t, "--content", content, "--quiet", "--includes")
	out, err := os.ReadFile(filepath.Join(content, "page.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	if !strings.Contains(html, "Kept line.") || strings.Contains(html, "Before.") || strings.Contains(html, "After.") {
		t.Errorf("expected only the region, got %q", html)
	}
}

// An include the render could not expand is printed once, by the render, and
// still fails -lint-strict.
func TestLint_StrictFailsOnAnUnresolvedInclude(t *testing.T) {
	content := filepath.Join(t.TempDir(), "content")
	writeFile(t, filepath.Join(content, "page.crv"), "{{ missing.txt }}\n")
	logged, err := runCapture(t, "--content", content, "--quiet", "--includes", "--lint-strict")
	if err == nil {
		t.Fatalf("expected -lint-strict to fail, got %q", logged)
	}
	if n := strings.Count(logged, "include-unresolved"); n != 1 {
		t.Errorf("expected one report, got %d in %q", n, logged)
	}
}
