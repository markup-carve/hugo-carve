package convert

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	carve "github.com/markup-carve/carve-go"
)

// What this repository itself does to a document, measured against the engine
// it is holding.
//
// TestSpecCorpus and scripts/corpus-drift.sh answer a different question, and
// only that one: how far the ENGINE is from the spec, and how much of that
// distance the go.mod pin is responsible for. Both run every document through
// Convert, and the verdict is the DIFFERENCE between two such runs - so a
// defect in this package cancels out of it exactly. Both runs render the same
// document wrongly, the set difference is empty, and corpus-drift.sh prints
// "the pin costs no documents" and exits 0 while attributing the damage to
// carve-go's embedded wasm trailing carve-rs.
//
// Measured, not reasoned about. Replacing every `<p>` with `<p class="...">` on
// the way out of ConvertWithOptions leaves the whole of CI green: `go test ./...`
// passes, because the hand-written cases in convert_test.go do not pin
// paragraph markup, and corpus-drift.sh reports 873 of 1190 documents diverging
// through BOTH the pin and carve-go main, calls that someone else's lag, and
// exits 0. Nothing in this repository could tell that from a genuine engine
// gap.
//
// This is the half that was missing, and it is deliberately not a second
// spec-conformance run. It compares Convert against the SAME linked engine, so
// engine lag cancels out of it completely: it cannot go red because carve-go is
// behind carve-rs, and it cannot go green because this package and the engine
// are wrong in the same way. It answers only "does this package hand the engine
// the right bytes and hand back what it got", which is the entire contribution
// this repository makes to a rendered page.
//
// Corpus-gated like TestSpecCorpus, so a plain checkout without the spec beside
// it still runs `go test ./...`. TestTheCorpusJobRunsEveryCorpusGatedTest is
// what keeps that skip from becoming a silent one.
func TestConvertAddsNothingToTheEngine(t *testing.T) {
	dir := os.Getenv("CARVE_SPEC_CORPUS")
	if dir == "" {
		t.Skip("CARVE_SPEC_CORPUS not set; see .github/workflows/ci.yml for the corpus job")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("CARVE_SPEC_CORPUS=%s is not readable: %v", dir, err)
	}

	// Every document, and not only the ones without front matter. The front
	// matter split is this package's other half, and TestSpecCorpus has to set
	// those 49 documents aside precisely because it cannot tell a correct split
	// from a wrong one - the spec's .html has no opinion about it. Here the
	// expectation is computed from the split itself, so they are checked rather
	// than excused.
	paired := 0
	var lost, rendered, reassembled []string

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".crv") {
			continue
		}
		base := strings.TrimSuffix(name, ".crv")
		blob, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		// Pairing matches TestSpecCorpus so both read the same population and
		// requireWholeCorpus means the same thing in both.
		if _, err := os.Stat(filepath.Join(dir, base+".html")); err != nil {
			continue
		}
		paired++
		source := string(blob)

		// The split loses nothing and invents nothing. A front matter block is
		// returned verbatim, so the two halves have to reconstitute the input
		// byte for byte - the cheapest possible statement that a document was
		// not silently truncated on the way in.
		fm, body := splitFrontMatter(source)
		if fm+body != source {
			lost = append(lost, base)
			continue
		}

		got, err := Convert(source)
		if err != nil {
			rendered = append(rendered, base+": convert error: "+err.Error())
			continue
		}

		want, err := carve.ToHTMLOptions(body, carve.Options{})
		if err != nil {
			// The engine refusing the body Convert would have handed it is a
			// finding about this package's split, not about the engine.
			rendered = append(rendered, base+": the engine rejected the body this package split out: "+err.Error())
			continue
		}
		if got.BodyHTML != strings.TrimRight(want, "\n") {
			rendered = append(rendered, base)
			continue
		}
		if got.FrontMatter != fm {
			reassembled = append(reassembled, base+": front matter not preserved verbatim")
			continue
		}
		// The assembled file still ends in the body that was just verified, and
		// still opens with the front matter. Checked as prefix and suffix rather
		// than by rebuilding the expected string, which would only restate
		// ConvertWithOptions and agree with it by construction.
		if !strings.HasPrefix(got.Output, fm) || !strings.HasSuffix(got.Output, got.BodyHTML+"\n") {
			reassembled = append(reassembled, base+": output does not open with the front matter and end with the body")
		}
	}

	// The same population guard TestSpecCorpus uses, for the same reason: an
	// empty or truncated corpus otherwise reports nothing wrong and passes.
	requireWholeCorpus(t, dir, paired, "corpus pairs read")

	sort.Strings(lost)
	sort.Strings(rendered)
	sort.Strings(reassembled)

	t.Logf("identity-checked=%d", paired)

	if len(lost) > 0 {
		t.Errorf("%d of %d documents do not survive the front matter split byte for byte "+
			"(front matter + body != source):\n%s",
			len(lost), paired, strings.Join(lost, "\n"))
	}
	if len(rendered) > 0 {
		t.Errorf("%d of %d documents come out of Convert differently from what the LINKED engine "+
			"returns for the same body. Engine lag cannot cause this - both sides are the same "+
			"carve-go build - so the defect is in this package, not in the pin:\n%s",
			len(rendered), paired, strings.Join(rendered, "\n"))
	}
	if len(reassembled) > 0 {
		t.Errorf("%d of %d documents are reassembled wrongly:\n%s",
			len(reassembled), paired, strings.Join(reassembled, "\n"))
	}
}
