package convert

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// How many documents the spec corpus should hold, derived rather than recorded.
//
// ONE spelling of "a corpus runner must not report success over an empty or
// short population" (the variant-2 defect catalogued in markup-carve/carve#755,
// noted for this repo inside markup-carve/hugo-carve#7).
//
// corpus_test.go guarded the population with `total < 400` and the comment "the
// corpus has 690 or more". The corpus holds 1131. That is not a rounding error,
// it is a guard that accepts a corpus missing two thirds of its documents, and
// truncation is the exact failure it exists to catch: measured on a sibling
// binding carrying the identical floor, a corpus cut to 420 documents reported
// every remaining document byte-identical and read as a clean run, with all 45
// then-diverging documents simply absent.
//
// THE COMPARISON HAS TO BE AGAINST SOMETHING THE RUNNER DOES NOT ITSELF READ AS
// THE POPULATION. Deriving "how many documents should there be" from the
// directory being checked would be a check that reads its own input hiding
// inside a fix for a check that cannot fail - emptying the directory would move
// both sides and the guard would still pass.
//
// So the reference is the corpus's SOURCE, not the corpus. tests/corpus is
// generated from the `::: compare` blocks in
// resources/examples/{core,extensions,edge-cases}.md (see
// scripts/generate-corpus.mjs in the spec repository); the generator refuses to
// write a corpus where the two disagree. Both live in the same spec checkout
// engine-drift.yml already clones, one directory away from CARVE_SPEC_CORPUS.
//
// Counting the source rather than recording a number also means there is no
// literal left to go stale: adding an example moves the expectation on the next
// run, without anyone editing this file. A hardcoded 1131 would be this same
// defect with a bigger number.
//
// This is the approach markup-carve/carve-go arrived at in its
// corpus_population_test.go, ported rather than reinvented.

// The pages the corpus is generated from, in the order the generator reads
// them. Order is irrelevant to a count; the list is the generator's.
var specExamplePages = []string{"core.md", "extensions.md", "edge-cases.md"}

// Mirrors generate-corpus.mjs: `::: compare`, or a longer colon run, with
// optional modifiers such as `::: compare no-render`.
var compareOpenLine = regexp.MustCompile(`^:{3,}\s+compare(\s+\S.*)?$`)

var compareMarkerRun = regexp.MustCompile(`^:{3,}`)

// declaredCorpusSize counts the example pairs the spec DECLARES, by reading the
// pages tests/corpus is generated from. corpusDir is CARVE_SPEC_CORPUS, i.e.
// <spec>/tests/corpus.
//
// The scan mirrors the generator's state machine rather than grepping: a
// `::: compare` line inside an already-open compare block is content, not a
// second pair, and the generator closes a block on a bare marker line.
// Mirroring keeps the two counts equal by construction instead of by luck.
func declaredCorpusSize(t *testing.T, corpusDir string) int {
	t.Helper()
	examplesDir := filepath.Join(corpusDir, "..", "..", "resources", "examples")
	declared := 0
	for _, page := range specExamplePages {
		path := filepath.Join(examplesDir, page)
		blob, err := os.ReadFile(path)
		if err != nil {
			// Not a soft skip. Without this file there is no independent
			// statement of how big the corpus should be, and a corpus check
			// with nothing to compare against is the failure shape this helper
			// exists to remove.
			t.Fatalf("no corpus source page at %s: %v. tests/corpus is generated from these pages; "+
				"if the spec moved them, this helper has to move with them", path, err)
		}
		inCompare := false
		marker := ""
		for _, line := range strings.Split(string(blob), "\n") {
			trimmed := strings.TrimSpace(line)
			if inCompare {
				if trimmed == marker {
					inCompare = false
					marker = ""
				}
				continue
			}
			if compareOpenLine.MatchString(trimmed) {
				declared++
				inCompare = true
				marker = compareMarkerRun.FindString(trimmed)
			}
		}
	}
	if declared == 0 {
		t.Fatalf("the corpus source pages under %s declare no ::: compare blocks at all; "+
			"this is a wiring problem, not a corpus of size zero", examplesDir)
	}
	return declared
}

// requireWholeCorpus is the only place this package decides whether a corpus
// population is big enough to draw a conclusion from. got is the number of
// documents the caller actually READ; what names it for the failure message.
//
// Equality rather than a floor, deliberately. A floor answers the wrong
// question: "at least 400" cannot tell a whole corpus from a truncated
// checkout, and truncation is the failure being guarded against.
func requireWholeCorpus(t *testing.T, corpusDir string, got int, what string) {
	t.Helper()
	declared := declaredCorpusSize(t, corpusDir)
	if got != declared {
		t.Fatalf("%s: %d, but the spec's example pages declare %d. Every ::: compare block in "+
			"resources/examples/{core,extensions,edge-cases}.md becomes one corpus pair, so a difference "+
			"means the corpus at %s is not the one those pages describe - a truncated or stale "+
			"checkout, a wrong CARVE_SPEC_CORPUS, or a corpus that needs regenerating "+
			"(npm run corpus:build in the spec repository). It does not mean this run was clean.",
			what, got, declared, corpusDir)
	}
}
