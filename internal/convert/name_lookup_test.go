package convert

import (
	"path/filepath"
	"strings"
	"testing"
)

// Every name lookup compares case exactly (carve 0.1.8, markup-carve/carve#2732).
//
// This is the one delta in that release a Hugo reader sees directly: a
// cross-reference whose case does not match its target used to render as a
// working link and now renders as the literal source text, so a page silently
// loses a link rather than following a wrong one.
//
// Measured across the pin move in separate processes: on carve-go v0.1.3
// (engine carve-rs d7837249) `</#plan>` against `{#Plan}` rendered
// `<a href="#Plan">Plan</a>`; on v0.1.5 (engine ade4db35) it renders
// `&lt;/#plan&gt;`. Both of these cases therefore fail on the previous pin,
// which is what makes them an answer to the delta rather than a restatement
// of current behavior.
//
// Driven through Convert rather than through carve-go, because what this
// repository owes is that the bytes reach the engine and come back untouched -
// the front matter split sits between them.

func TestNameLookup_AnExactlyCasedCrossReferenceStillLinks(t *testing.T) {
	res, err := Convert("{#Plan}\n# Plan\n\nSee </#Plan>.\n")
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(res.BodyHTML, `<a href="#Plan">`) {
		t.Errorf("an exactly-cased cross-reference should still be a link:\n%s", res.BodyHTML)
	}
}

func TestNameLookup_AWronglyCasedCrossReferenceIsLiteral(t *testing.T) {
	res, err := Convert("{#Plan}\n# Plan\n\nSee </#plan>.\n")
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if strings.Contains(res.BodyHTML, `href="#Plan"`) {
		t.Errorf("a wrongly-cased cross-reference must not resolve to the differently-cased id:\n%s", res.BodyHTML)
	}
	if !strings.Contains(res.BodyHTML, "&lt;/#plan&gt;") {
		t.Errorf("a wrongly-cased cross-reference should render as literal text:\n%s", res.BodyHTML)
	}
}

// A collapsed reference falling back to heading text, the other spelling the
// clause names. `[plan][]` resolved against the heading `Plan` before 0.1.8.
func TestNameLookup_AWronglyCasedCollapsedReferenceIsLiteral(t *testing.T) {
	res, err := Convert("See [Plan][] and [plan][].\n\n# Plan\n")
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(res.BodyHTML, `<a href="#Plan">Plan</a>`) {
		t.Errorf("the exactly-cased collapsed reference should link:\n%s", res.BodyHTML)
	}
	if !strings.Contains(res.BodyHTML, "[plan][]") {
		t.Errorf("the wrongly-cased collapsed reference should stay literal:\n%s", res.BodyHTML)
	}
}

// The include selector reads the same way, and already did: measured on both
// pins, `{{ sub/frag.crv #alpha }}` against the id `Alpha` left the directive
// literal on carve-go v0.1.3 as well. Pinned here so the two rules cannot
// drift apart later, and recorded as unchanged rather than claimed as new.
func TestNameLookup_AnIncludeSelectorComparesCaseExactly(t *testing.T) {
	s := newSite(t)
	write(t, filepath.Join(s.content, "sub", "cased.crv"), "# Alpha\n\nAlpha body.\n")

	body, opts := s.page(t, "exact.crv", "{{ sub/cased.crv #Alpha }}\n")
	res, err := ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, "Alpha body.") {
		t.Errorf("an exactly-cased selector should select the section:\n%s", res.BodyHTML)
	}

	body, opts = s.page(t, "wrong.crv", "{{ sub/cased.crv #alpha }}\n")
	res, err = ConvertWithOptions(body, opts)
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if strings.Contains(res.BodyHTML, "Alpha body.") {
		t.Errorf("a wrongly-cased selector must not select the section:\n%s", res.BodyHTML)
	}
	// The engine emits the rule id "include-section". carve-go's own
	// IncludeWarning.Rule doc comment lists "include-section-missing" instead,
	// which is markup-carve/carve-go's to reconcile; this asserts the value
	// measured from the pinned artifact.
	if len(res.Warnings) != 1 || res.Warnings[0].Rule != "include-section" {
		t.Errorf("expected one include-section warning, got %v", res.Warnings)
	}
}
