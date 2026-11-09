package convert

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
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
// resources/examples/{core,extensions}.md and resources/examples/edge-cases/ (see
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

// specExamplePaths lists the page files in examplesDir. Since
// markup-carve/carve#4132 the edge cases are one file per section under
// edge-cases/; an older spec checkout still has the single edge-cases.md.
func specExamplePaths(t *testing.T, examplesDir string) []string {
	t.Helper()
	paths := []string{}
	for _, page := range specExamplePages {
		sectionDir := filepath.Join(examplesDir, strings.TrimSuffix(page, ".md"))
		entries, err := os.ReadDir(sectionDir)
		if err != nil {
			paths = append(paths, filepath.Join(examplesDir, page))
			continue
		}
		sections := []string{}
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() && strings.HasSuffix(name, ".md") && name != "README.md" {
				sections = append(sections, filepath.Join(sectionDir, name))
			}
		}
		if len(sections) == 0 {
			t.Fatalf("no section files in %s", sectionDir)
		}
		sort.Strings(sections)
		paths = append(paths, sections...)
	}
	return paths
}

// Mirrors generate-corpus.mjs: `::: compare`, or a longer colon run, with
// optional modifiers such as `::: compare no-render`.
var compareOpenLine = regexp.MustCompile(`^:{3,}\s+compare(\s+\S.*)?$`)

var compareMarkerRun = regexp.MustCompile(`^:{3,}`)

// declaredCorpusSize counts the example pairs the spec DECLARES, by reading the
// pages tests/corpus is generated from. corpusDir is CARVE_SPEC_CORPUS, i.e.
// <spec>/tests/corpus.
//
// Every `carve` fence inside a compare block is one pair; a block may hold several.
func declaredCorpusSize(t *testing.T, corpusDir string) int {
	t.Helper()
	examplesDir := filepath.Join(corpusDir, "..", "..", "resources", "examples")
	// The generator writes one fixture per distinct input (markup-carve/carve#5244).
	inputs := map[string]bool{}
	for _, path := range specExamplePaths(t, examplesDir) {
		blob, err := os.ReadFile(path)
		if err != nil {
			// Not a soft skip. Without this file there is no independent
			// statement of how big the corpus should be, and a corpus check
			// with nothing to compare against is the failure shape this helper
			// exists to remove.
			t.Fatalf("no corpus source page at %s: %v. tests/corpus is generated from these pages; "+
				"if the spec moved them, this helper has to move with them", path, err)
		}
		for _, input := range declaredInputs(strings.Split(string(blob), "\n")) {
			inputs[input] = true
		}
	}
	declared := len(inputs)
	if declared == 0 {
		t.Fatalf("the corpus source pages under %s declare no ::: compare blocks at all; "+
			"this is a wiring problem, not a corpus of size zero", examplesDir)
	}
	return declared
}

// countDeclaredPairs ports the spec's scripts/lib/example-pair-census.mjs.
// Nothing inside an open fence is markup, so a ```` example holding a
// ``` carve line declares no pair of its own.
func countDeclaredPairs(lines []string) int {
	return len(declaredInputs(lines))
}

// declaredInputs returns the fixture each declared pair writes, after the
// generator's applyModifiers.
func declaredInputs(lines []string) []string {
	var inputs, body, modifiers []string
	inCarve := false
	marker, fence := "", ""
	for _, line := range lines {
		if fence != "" {
			if strings.HasPrefix(line, fence) && strings.TrimSpace(line[len(fence):]) == "" {
				fence = ""
				if inCarve {
					inputs = append(inputs, fixtureInput(strings.Join(body, "\n"), modifiers))
					inCarve = false
				}
			} else if inCarve {
				body = append(body, line)
			}
			continue
		}
		ticks := len(line) - len(strings.TrimLeft(line, "`"))
		if ticks >= 3 {
			fence = line[:ticks]
			if marker != "" && strings.TrimSpace(line[ticks:]) == "carve" {
				inCarve, body = true, nil
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if marker == "" {
			if compareOpenLine.MatchString(trimmed) {
				marker = compareMarkerRun.FindString(trimmed)
				modifiers = strings.Fields(trimmed)[2:]
			}
		} else if trimmed == marker {
			marker = ""
		}
	}
	return inputs
}

func fixtureInput(carve string, modifiers []string) string {
	out := carve + "\n"
	if slices.Contains(modifiers, "no-final-newline") {
		out = strings.TrimSuffix(out, "\n")
	}
	if slices.Contains(modifiers, "crlf") {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	if slices.Contains(modifiers, "cr") {
		out = strings.ReplaceAll(out, "\n", "\r")
	}
	if slices.Contains(modifiers, "bom") {
		out = "\ufeff" + out
	}
	if slices.Contains(modifiers, "nul") {
		out = strings.ReplaceAll(out, "\u2400", "\x00")
	}
	return out
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
			"resources/examples/{core,extensions}.md and edge-cases/ yields one corpus pair per carve fence, so a difference "+
			"means the corpus at %s is not the one those pages describe - a truncated or stale "+
			"checkout, a wrong CARVE_SPEC_CORPUS, or a corpus that needs regenerating "+
			"(npm run corpus:build in the spec repository). It does not mean this run was clean.",
			what, got, declared, corpusDir)
	}
}

func TestDeclaredPairsCountEveryCarveFenceInABlock(t *testing.T) {
	page := strings.Join([]string{
		"::: compare",
		"```carve", "one", "```", "```html", "<p>one</p>", "```",
		"````carve", "```carve", "nested, not a pair", "```", "````", "```html", "<pre>two</pre>", "```",
		"```carve", "three", "```", "```html", "<p>three</p>", "```",
		":::",
		"```carve", "outside any block", "```",
	}, "\n")
	if got := countDeclaredPairs(strings.Split(page, "\n")); got != 3 {
		t.Fatalf("got %d pairs, want 3", got)
	}
}
