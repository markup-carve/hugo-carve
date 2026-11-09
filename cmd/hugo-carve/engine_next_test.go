package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Behavior that arrives with the engine after carve-rs 0.1.13.

const captionAfterTable = "|= A |\n| x |\n\nTable 1: caption\n"

func TestCaptionHints_OffWithoutTheFlag(t *testing.T) {
	content := lintSite(t, captionAfterTable)
	logged, err := runCapture(t, "--content", content, "--quiet", "--lint")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if strings.Contains(logged, "caption-looking-paragraph") {
		t.Errorf("caption hint reported without -caption-hints: %q", logged)
	}
}

func TestCaptionHints_ReportsAndImpliesLint(t *testing.T) {
	content := lintSite(t, captionAfterTable)
	logged, err := runCapture(t, "--content", content, "--quiet", "--caption-hints")
	if err != nil {
		t.Fatalf("-caption-hints must not fail the run: %v", err)
	}
	want := filepath.Join(content, "page.crv") + ":4:1: caption-looking-paragraph: "
	if !strings.Contains(logged, want) {
		t.Errorf("expected %q in %q", want, logged)
	}
}

// carve-go reports no severity, so a default-on hint counts like any other
// finding under -lint-strict.
func TestLintStrict_CountsDefaultOnHints(t *testing.T) {
	content := lintSite(t, "a\n\n---\n\nb\n")
	logged, err := runCapture(t, "--content", content, "--quiet", "--lint-strict")
	if err == nil {
		t.Fatalf("-lint-strict passed a page with a --- thematic break: %q", logged)
	}
	want := filepath.Join(content, "page.crv") + ":3:1: thematic-break-spelling: "
	if !strings.Contains(logged, want) {
		t.Errorf("expected %q in %q", want, logged)
	}
}

func TestLintStrict_CanonicalBreakIsClean(t *testing.T) {
	content := lintSite(t, "a\n\n***\n\nb\n")
	logged, err := runCapture(t, "--content", content, "--quiet", "--lint-strict")
	if err != nil {
		t.Fatalf("-lint-strict failed a page with a *** break: %v, %q", err, logged)
	}
}

func TestIncludes_TableRowsAndColumns(t *testing.T) {
	content := filepath.Join(t.TempDir(), "content")
	writeFile(t, filepath.Join(content, "data", "plans.csv"),
		"Plan,Seats,Price\nStarter,1,0\n\"Team, annual\",10,99\nEnterprise,100,999\n")
	writeFile(t, filepath.Join(content, "page.crv"),
		"{table}\n{{ data/plans.csv @code:csv @rows:2-3 @cols:\"Seats,Plan\" }}\n")
	runCLI(t, "--content", content, "--includes", "--quiet")

	got := readPage(t, filepath.Join(content, "page.html"))
	for _, want := range []string{
		`<tr><th scope="col">Seats</th><th scope="col">Plan</th></tr>`,
		`<tr><td>1</td><td>Starter</td></tr>`,
		`<tr><td>10</td><td>Team, annual</td></tr>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"Enterprise", "Price", "{{"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("%q should not be in:\n%s", unwanted, got)
		}
	}
}

// carve-rs 0.1.13 panics on this body (spec corpus row 674).
func TestRun_BracedOpenerBeforeTerminalBackslashConverts(t *testing.T) {
	content := filepath.Join(t.TempDir(), "content")
	writeFile(t, filepath.Join(content, "page.crv"), "{*}\\\n")
	runCLI(t, "--content", content, "--quiet")

	got := readPage(t, filepath.Join(content, "page.html"))
	if !strings.Contains(got, "<p>{*}<br>\n</p>") {
		t.Errorf("unexpected render:\n%s", got)
	}
}
