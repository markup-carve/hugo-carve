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
