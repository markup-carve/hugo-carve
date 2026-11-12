package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestReadmeQuickStartBuildsASite follows the README as a new user would:
// scaffold a site with `hugo new site`, add the README's Hugo config, convert
// a page, run `hugo` (hugo-carve#56). It needs a hugo binary; CI sets HUGO_CARVE_E2E so a
// missing one fails there instead of skipping.
func TestReadmeQuickStartBuildsASite(t *testing.T) {
	hugo, err := exec.LookPath("hugo")
	if err != nil {
		if os.Getenv("HUGO_CARVE_E2E") != "" {
			t.Fatal("HUGO_CARVE_E2E is set but hugo is not on PATH")
		}
		t.Skip("hugo not on PATH")
	}

	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile("(?s)## Usage\n.*?```toml\n(.*?)```").FindSubmatch(readme)
	if m == nil {
		t.Fatal("README Usage section has no toml block with the Hugo config")
	}

	site := filepath.Join(t.TempDir(), "site")
	hugoRun(t, "", hugo, "new", "site", site)
	config := filepath.Join(site, "hugo.toml")
	scaffolded, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	write(t, config, string(scaffolded)+"\n"+string(m[1]))
	writeFile(t, filepath.Join(site, "layouts", "single.html"),
		"<!doctype html><title>{{ .Title }}</title>{{ .Content }}\n")
	writeFile(t, filepath.Join(site, "content", "demo.crv"),
		"---\ntitle: Carve Feature Demo\ndate: 2026-06-02\n---\n\n# Carve Feature Demo\n\nSome *bold* text.\n")

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	if err := run([]string{"--content", filepath.Join(site, "content")}, devNull, devNull); err != nil {
		t.Fatalf("hugo-carve: %v", err)
	}
	hugoRun(t, site, hugo)

	page, err := os.ReadFile(filepath.Join(site, "public", "demo", "index.html"))
	if err != nil {
		t.Fatalf("hugo wrote no page: %v", err)
	}
	for _, want := range []string{"<title>Carve Feature Demo</title>", "<strong>bold</strong>"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("page lacks %q:\n%s", want, page)
		}
	}
	if _, err := os.Stat(filepath.Join(site, "public", "demo.crv")); err == nil {
		t.Error("the raw .crv source was published")
	}
}

func hugoRun(t *testing.T, dir, hugo string, args ...string) {
	t.Helper()
	cmd := exec.Command(hugo, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hugo %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
