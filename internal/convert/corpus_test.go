package convert

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The mandatory spec corpus, driven through Convert - the product path, front
// matter split and all - rather than through carve-go directly.
//
// This repo renders with an engine it never builds: go.mod pins a carve-go
// version, and that carve-go embeds a prebuilt wasm compiled from some
// carve-rs commit. Two pins deep, both invisible from here, and neither moves
// on its own. Everything else in this package asserts hand-written
// expectations, which a stale engine satisfies exactly as well as a current
// one - that is how carve-go's own embedded wasm reached 36 of 610 documents
// wrong with a green tick (carve#735, markup-carve/carve-go#26).
//
// A version distance cannot answer this. 200 carve-rs commits can change
// nothing, and one can change a construct. This counts DOCUMENTS.
//
// CARVE_SPEC_CORPUS gives the corpus directory, matching carve-go. When unset
// the test skips, so `go test ./...` works in a plain checkout without the
// spec. CARVE_CORPUS_MAX_DIVERGENCE is how many documents may diverge before
// this fails; it defaults to 0, and engine-drift.yml sets it deliberately when
// it runs this twice to separate the two lags (see that file).
func TestSpecCorpus(t *testing.T) {
	dir := os.Getenv("CARVE_SPEC_CORPUS")
	if dir == "" {
		t.Skip("CARVE_SPEC_CORPUS not set; see .github/workflows/engine-drift.yml for the corpus job")
	}

	tolerance := 0
	if raw := os.Getenv("CARVE_CORPUS_MAX_DIVERGENCE"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("CARVE_CORPUS_MAX_DIVERGENCE=%q is not a number", raw)
		}
		tolerance = parsed
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("CARVE_SPEC_CORPUS=%s is not readable: %v", dir, err)
	}

	// Names only, and never anything else: engine-drift.yml compares two runs'
	// lists with `comm`, so a key that varies with an error message would make
	// the same document look like two different ones and manufacture a
	// pin-only entry. Human-readable detail goes in `details`, keyed the same.
	var mismatches []string
	details := map[string]string{}
	total := 0
	// A corpus document opening with `---`, `+++` or `{` is indistinguishable
	// from a front matter block, and splitting it is correct behavior for a
	// Hugo content file. Those documents are not an engine measurement, so
	// they are counted and reported rather than compared - mixing them in
	// would put 35 documents of ambiguity into a number that is supposed to
	// mean "the engine disagrees with the spec".
	frontMatterClaimed := 0

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".crv") {
			continue
		}
		base := strings.TrimSuffix(name, ".crv")
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want, err := os.ReadFile(filepath.Join(dir, base+".html"))
		if err != nil {
			// A .crv with no .html pair is not an input to this comparison.
			continue
		}

		got, err := Convert(string(src))
		if err != nil {
			total++
			mismatches = append(mismatches, base)
			details[base] = "convert error: " + err.Error()
			continue
		}
		if got.FrontMatter != "" {
			frontMatterClaimed++
			continue
		}
		total++
		if strings.TrimRight(got.BodyHTML, "\n") != strings.TrimRight(string(want), "\n") {
			mismatches = append(mismatches, base)
		}
	}

	// Without this, an empty or wrong directory would report zero mismatches
	// and pass - the exact shape of check that let the stale artifact through.
	if total < 400 {
		t.Fatalf("only %d comparable corpus pairs found in %s; the corpus has 690 or more, so this is a wiring problem, not a clean run", total, dir)
	}

	sort.Strings(mismatches)

	// Machine-readable, because engine-drift.yml reads these back out of the
	// `-v` log to compare two runs against each other. The NAMES matter, not
	// just the count: two runs can diverge on the same number of documents and
	// on different ones, and subtracting the counts would report that as "the
	// pin costs nothing".
	t.Logf("comparable=%d", total)
	t.Logf("divergent=%d", len(mismatches))
	t.Logf("front-matter-claimed=%d", frontMatterClaimed)
	t.Logf("divergent-list=%s", strings.Join(mismatches, ","))

	if len(mismatches) > tolerance {
		reported := make([]string, 0, len(mismatches))
		for _, name := range mismatches {
			if detail, ok := details[name]; ok {
				reported = append(reported, name+": "+detail)
				continue
			}
			reported = append(reported, name)
		}
		t.Fatalf("%d of %d corpus documents render differently through Convert, over the tolerance of %d.\n"+
			"The engine is two pins deep - go.mod pins carve-go, which embeds a wasm built from carve-rs -\n"+
			"so the usual cause is a stale go.mod pin rather than anything in this package.\n%s",
			len(mismatches), total, tolerance, strings.Join(reported, "\n"))
	}
}
