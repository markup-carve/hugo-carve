package convert

import (
	"os"
	"path/filepath"
	"sort"
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
// The corpus path comes from CARVE_SPEC_CORPUS, matching carve-go. When unset
// the test skips, so `go test ./...` works in a plain checkout without the
// spec; CI always sets it, and the guard below turns "the corpus wasn't really
// there" into a failure rather than a pass.
func TestSpecCorpus(t *testing.T) {
	dir := os.Getenv("CARVE_SPEC_CORPUS")
	if dir == "" {
		t.Skip("CARVE_SPEC_CORPUS not set; see .github/workflows/ci.yml for the corpus job")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("CARVE_SPEC_CORPUS=%s is not readable: %v", dir, err)
	}

	var mismatches []string
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
			mismatches = append(mismatches, base+": convert error: "+err.Error())
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

	t.Logf("%d of %d comparable corpus documents render identically (%d more opened with a front matter delimiter and were split rather than compared)",
		total-len(mismatches), total, frontMatterClaimed)

	sort.Strings(mismatches)
	if len(mismatches) > 0 {
		t.Fatalf("%d of %d corpus documents render differently through Convert.\n"+
			"The engine is two pins deep - go.mod pins carve-go, which embeds a wasm built from carve-rs -\n"+
			"so the usual cause is a stale go.mod pin rather than anything in this package.\n%s",
			len(mismatches), total, strings.Join(mismatches, "\n"))
	}
}
