package convert

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	carve "github.com/markup-carve/carve-go"
)

// The Carve 0.2 conformance corpus, driven through Convert.
//
// carve `main` has no tests/corpus. Its corpus is spec/0.2/conformance:
// examples.json (every example of the spec documents) and clause-cases.json
// (inputs a clause decides that no example holds). This test renders every
// `render` entry of both that states `expected.html` and compares the HTML byte
// for byte, which is what TestSpecCorpus does for the 0.1 corpus.
//
// CARVE_CONFORMANCE gives the conformance directory. Unset, the test skips.
// CARVE_CORPUS_MAX_DIVERGENCE is the tolerance, as for TestSpecCorpus.
//
// An entry is compared only when its options map onto carve.Options: carve-go
// has no field for some engine options (softBreak, headingShift, social, ...),
// and rendering without them would count a binding gap as an engine divergence.
// Those entries are counted and listed by option, never dropped silently.

// conformanceEntry is the part of an examples.json or clause-cases.json entry
// this test reads.
type conformanceEntry struct {
	ID         string                     `json:"id"`
	Document   string                     `json:"document"`
	Kind       string                     `json:"kind"`
	Input      *string                    `json:"input"`
	InputBytes string                     `json:"inputBytes"`
	Files      json.RawMessage            `json:"files"`
	Tree       json.RawMessage            `json:"tree"`
	Options    map[string]json.RawMessage `json:"options"`
	Expected   struct {
		HTML *string `json:"html"`
	} `json:"expected"`
}

// conformanceOptions maps an entry's options (the names of the spec's
// data/options.json) onto carve.Options. It returns the option names carve-go
// cannot express; a non-empty list means the entry is not comparable.
func conformanceOptions(raw map[string]json.RawMessage) (carve.Options, []string) {
	var out carve.Options
	var unsupported []string
	for name, value := range raw {
		var err error
		switch name {
		case "rawHtml":
			err = json.Unmarshal(value, &out.RawHTML)
		case "mode":
			err = json.Unmarshal(value, &out.Mode)
		case "profile":
			err = json.Unmarshal(value, &out.Profile)
		case "headingIds":
			err = json.Unmarshal(value, &out.HeadingIDs)
		case "symbols":
			err = json.Unmarshal(value, &out.Symbols)
		case "strictLosses":
			err = json.Unmarshal(value, &out.StrictLosses)
		case "roundtrip":
			err = json.Unmarshal(value, &out.Roundtrip)
		case "sourceLines":
			err = json.Unmarshal(value, &out.SourceLines)
		case "extensions":
			err = json.Unmarshal(value, &out.Extensions)
		case "urlPolicy":
			out.URLPolicy = &carve.URLPolicy{}
			err = json.Unmarshal(value, out.URLPolicy)
		case "renderers":
			// carve-go takes only the bare "standard" renderer; a renderer
			// with its own options has no form there.
			err = json.Unmarshal(value, &out.Renderers)
		default:
			unsupported = append(unsupported, name)
			continue
		}
		if err != nil {
			unsupported = append(unsupported, name)
		}
	}
	sort.Strings(unsupported)
	return out, unsupported
}

func loadConformance(t *testing.T, path, list string) []conformanceEntry {
	t.Helper()
	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("CARVE_CONFORMANCE: %v", err)
	}
	var file map[string]json.RawMessage
	if err := json.Unmarshal(blob, &file); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var entries []conformanceEntry
	if err := json.Unmarshal(file[list], &entries); err != nil {
		t.Fatalf("%s: the %q list: %v", path, list, err)
	}
	return entries
}

// requireWholeExamples checks examples.json against the documents it is
// generated from: each document in spec/0.2 must hold as many `Example N`
// headings as the file has entries for it. The reference is the source the
// generator reads, one directory up, so a truncated or stale examples.json
// fails here instead of reading as a clean run.
func requireWholeExamples(t *testing.T, conformanceDir string, examples []conformanceEntry) {
	t.Helper()
	perDocument := map[string]int{}
	for _, e := range examples {
		perDocument[e.Document]++
	}
	if len(perDocument) == 0 {
		t.Fatalf("%s/examples.json holds no examples", conformanceDir)
	}
	heading := regexp.MustCompile(`(?m)^Example [0-9]+(?: |$)`)
	specDir := filepath.Dir(filepath.Clean(conformanceDir))
	for document := range perDocument {
		if _, err := os.Stat(filepath.Join(specDir, document)); err != nil {
			t.Fatalf("examples.json names %s, which %s does not hold: %v", document, specDir, err)
		}
	}
	// Every document with examples, so one missing from examples.json
	// entirely fails too.
	documents, err := filepath.Glob(filepath.Join(specDir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	var short []string
	for _, path := range documents {
		blob, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		document := filepath.Base(path)
		want, got := len(heading.FindAll(blob, -1)), perDocument[document]
		if want != got {
			short = append(short, fmt.Sprintf("%s: %d examples in the document, %d in examples.json", document, want, got))
		}
	}
	if len(short) > 0 {
		sort.Strings(short)
		t.Fatalf("examples.json does not match the documents it is generated from:\n%s", strings.Join(short, "\n"))
	}
}

func TestConformance(t *testing.T) {
	dir := os.Getenv("CARVE_CONFORMANCE")
	if dir == "" {
		t.Skip("CARVE_CONFORMANCE not set; the conformance job in ci.yml sets it")
	}

	tolerance := 0
	if raw := os.Getenv("CARVE_CORPUS_MAX_DIVERGENCE"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("CARVE_CORPUS_MAX_DIVERGENCE=%q is not a number", raw)
		}
		tolerance = parsed
	}

	examples := loadConformance(t, filepath.Join(dir, "examples.json"), "examples")
	requireWholeExamples(t, dir, examples)
	cases := loadConformance(t, filepath.Join(dir, "clause-cases.json"), "cases")
	if len(cases) == 0 {
		t.Fatalf("%s/clause-cases.json holds no cases", dir)
	}

	var (
		mismatches  []string
		details     = map[string]string{}
		compared    int
		population  int
		frontMatter int
		unsupported = map[string][]string{}
		// A render entry with no expected.html (a refusal, diagnostics only,
		// htmlIncludes) or one the binding cannot feed (files, inputBytes,
		// tree) is out of scope for an HTML comparison through Convert.
		outOfScope = map[string]int{}
		// Entries the engine renders as expected and Convert does not.
		convertAdded []string
	)

	run := func(e conformanceEntry) {
		population++
		switch {
		case e.Expected.HTML == nil:
			outOfScope["no expected.html"]++
			return
		case e.Input == nil:
			outOfScope["no Carve input (tree or inputBytes)"]++
			return
		case len(e.Files) > 0:
			outOfScope["include files"]++
			return
		}
		opts, missing := conformanceOptions(e.Options)
		if len(missing) > 0 {
			key := strings.Join(missing, "+")
			unsupported[key] = append(unsupported[key], e.ID)
			return
		}
		got, err := convertWithEngineOptions(*e.Input, Options{}, opts)
		if err != nil {
			// A front matter block Hugo cannot read is refused by design.
			if fm, _ := splitFrontMatter(*e.Input); fm != "" {
				frontMatter++
				return
			}
			compared++
			mismatches = append(mismatches, e.ID)
			details[e.ID] = "convert error: " + err.Error()
			if direct, err := carve.ToHTMLOptions(*e.Input, opts); err == nil &&
				strings.TrimRight(direct, "\n") == strings.TrimRight(*e.Expected.HTML, "\n") {
				convertAdded = append(convertAdded, e.ID)
			}
			return
		}
		// As in TestSpecCorpus: an input that opens like front matter is
		// split off by design, which is not an engine measurement.
		if got.FrontMatter != "" {
			frontMatter++
			return
		}
		compared++
		want := strings.TrimRight(*e.Expected.HTML, "\n")
		if got.BodyHTML == want {
			return
		}
		mismatches = append(mismatches, e.ID)
		// The same input through the engine alone. Where the engine gets it
		// right and Convert does not, the defect is this package's, and no
		// engine pin can clear it: that fails at any tolerance.
		if direct, err := carve.ToHTMLOptions(*e.Input, opts); err == nil &&
			strings.TrimRight(direct, "\n") == want {
			convertAdded = append(convertAdded, e.ID)
		}
	}

	for _, e := range examples {
		if e.Kind == "render" {
			run(e)
		}
	}
	for _, e := range cases {
		// A clause case without kind is a render (conformance README).
		if e.Kind == "" || e.Kind == "render" {
			run(e)
		}
	}

	if compared == 0 {
		t.Fatalf("no conformance entry was compared out of %d render entries", population)
	}

	sort.Strings(mismatches)
	keys := make([]string, 0, len(unsupported))
	skipped := 0
	for key, ids := range unsupported {
		keys = append(keys, key)
		skipped += len(ids)
	}
	sort.Strings(keys)
	for _, key := range keys {
		t.Logf("unsupported option %s: %d (%s)", key, len(unsupported[key]), strings.Join(unsupported[key], ","))
	}
	scopes := make([]string, 0, len(outOfScope))
	for scope, n := range outOfScope {
		scopes = append(scopes, fmt.Sprintf("%s=%d", scope, n))
	}
	sort.Strings(scopes)

	// Read back by scripts/conformance-drift.sh, names and all; see
	// corpus-drift.sh for why a count alone is not enough.
	t.Logf("render-entries=%d", population)
	t.Logf("comparable=%d", compared)
	t.Logf("divergent=%d", len(mismatches))
	t.Logf("front-matter-claimed=%d", frontMatter)
	t.Logf("unsupported-options=%d", skipped)
	t.Logf("out-of-scope=%s", strings.Join(scopes, ", "))
	t.Logf("divergent-list=%s", strings.Join(mismatches, ","))

	if len(convertAdded) > 0 {
		t.Fatalf("Convert renders %d conformance entries differently from the engine it holds, which renders them as expected: %s",
			len(convertAdded), strings.Join(convertAdded, ","))
	}
	if len(mismatches) > tolerance {
		reported := make([]string, 0, len(mismatches))
		for _, id := range mismatches {
			if detail, ok := details[id]; ok {
				reported = append(reported, id+": "+detail)
				continue
			}
			reported = append(reported, id)
		}
		t.Fatalf("%d of %d conformance entries render differently through Convert, over the tolerance of %d:\n%s",
			len(mismatches), compared, tolerance, strings.Join(reported, "\n"))
	}
}
