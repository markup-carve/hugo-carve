package convert

import (
	"sort"
	"strings"
	"testing"

	carve "github.com/markup-carve/carve-go/v2"
)

// The spoiler construct renders differently with the extension on, which makes
// it a probe for whether a selection reached the engine rather than for whether
// a flag was stored.
const spoilerSource = "Plot: :spoiler[the butler did it].\n"

const (
	spoilerOn  = `class="spoiler`
	spoilerOff = `class="ext-spoiler"`
)

// engineRegistryKeys asks the ENGINE which keys exist, rather than restating
// the list this package ships. An unknown key is refused with the registry in
// the message, and that message is the only channel carve-go exposes for it:
// there is no exported list to read.
//
// If the message shape ever moves, this fails rather than returning a short
// list that would make the comparison below pass by having nothing to compare.
func engineRegistryKeys(t *testing.T) []string {
	t.Helper()
	_, err := carve.ToHTMLOptions("probe\n", carve.Options{Extensions: []string{"zz-not-an-extension"}})
	if err == nil {
		t.Fatal("the engine accepted a key that is not in any registry, so this helper cannot read the registry from its refusal")
	}
	msg := err.Error()
	const marker = "expected one of: "
	i := strings.Index(msg, marker)
	if i < 0 {
		t.Fatalf("the engine's refusal no longer carries %q, so the registry cannot be read from it. "+
			"Message was: %s", marker, msg)
	}
	list := strings.TrimSpace(msg[i+len(marker):])
	list = strings.TrimSuffix(list, ")")
	var keys []string
	for _, k := range strings.Split(list, ",") {
		if k = strings.TrimSpace(k); k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) < 2 {
		t.Fatalf("read %d keys out of the engine's refusal, which is too few to be the registry: %s", len(keys), msg)
	}
	return keys
}

// The bundle this package asks for is the whole registry, derived rather than
// recorded. A key added to the engine fails here instead of being quietly left
// out of -extensions, and a key removed fails here instead of making every page
// an error.
func TestBundleExtensionsIsTheWholeRegistry(t *testing.T) {
	t.Skip("reads the 0.1 registry from the engine's refusal message; 0.2 lists carve.StandardExtensions (TestBundleIsTheStandardList)")
	want := engineRegistryKeys(t)
	got := bundleExtensions()

	sort.Strings(want)
	sortedGot := append([]string(nil), got...)
	sort.Strings(sortedGot)

	if strings.Join(sortedGot, ",") != strings.Join(want, ",") {
		t.Errorf("the bundle this package sends is not the engine's registry.\n"+
			"missing from bundleExtensionKeys: %v\nnot in the engine registry: %v",
			missing(want, sortedGot), missing(sortedGot, want))
	}
}

func missing(from, in []string) []string {
	have := make(map[string]bool, len(in))
	for _, k := range in {
		have[k] = true
	}
	var out []string
	for _, k := range from {
		if !have[k] {
			out = append(out, k)
		}
	}
	return out
}

// bundleExtensions hands out a copy, so a caller cannot truncate the package's
// list through the slice it was given.
func TestBundleExtensionsIsNotAliased(t *testing.T) {
	first := bundleExtensions()
	if len(first) == 0 {
		t.Fatal("the bundle is empty")
	}
	first[0] = "clobbered"
	if bundleExtensions()[0] == "clobbered" {
		t.Error("bundleExtensions returns the package slice itself, so a caller can rewrite it")
	}
}

// What -extensions is for: the bundle reaches the engine and a construct that
// needs an extension renders as enabled.
//
// This is the case that failed outright on carve-go v0.1.5 while the name "all"
// was still being sent, with no page produced at all.
func TestExtensionsEnablesTheBundle(t *testing.T) {
	res, err := ConvertWithOptions(spoilerSource, Options{Extensions: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, spoilerOn) {
		t.Errorf("the spoiler extension did not reach the engine:\n%s", res.BodyHTML)
	}
}

// Leave one out: with the extension removed from the bundle, the same document
// has to come back unenabled. Without this, the test above would also pass for
// a bundle that happened to be the engine's default.
func TestTheBundleIsWhatEnablesTheExtension(t *testing.T) {
	t.Skip("leaves out the 0.1 key spoiler; 0.2 names it carve.spoiler (TestTheBundleIsWhatEnablesTheSpoiler)")
	var without []string
	for _, k := range bundleExtensionKeys {
		if k != "spoiler" {
			without = append(without, k)
		}
	}

	got, err := carve.ToHTMLOptions(spoilerSource, carve.Options{Extensions: without})
	if err != nil {
		t.Fatalf("ToHTMLOptions: %v", err)
	}
	if !strings.Contains(got, spoilerOff) {
		t.Errorf("dropping spoiler from the bundle changed nothing, so the selection is not what enables it:\n%s", got)
	}
}

// Off by default. The bundle is sent only when it was asked for.
func TestNoExtensionsLeavesTheBundleOff(t *testing.T) {
	res, err := ConvertWithOptions(spoilerSource, Options{})
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, spoilerOff) {
		t.Errorf("an extension construct rendered as enabled with no selection:\n%s", res.BodyHTML)
	}
}

// Static with no selection implies the bundle in carve-go, which is why
// ConvertWithOptions no longer populates Extensions for it.
func TestStaticStillEnablesTheBundle(t *testing.T) {
	t.Skip("0.2 static mode no longer implies -extensions (CHANGELOG Unreleased)")
	res, err := ConvertWithOptions(spoilerSource, Options{Static: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, spoilerOn) {
		t.Errorf("-static no longer enables the bundle:\n%s", res.BodyHTML)
	}
}

// TestLintAcceptsTheBundle: the bundle reaches the render, and the lint pass,
// which reads no extensions in 0.2 (C2-W-109), runs instead of refusing.
func TestLintAcceptsTheBundle(t *testing.T) {
	if _, err := Lint(spoilerSource, Options{Extensions: true}); err != nil {
		t.Fatalf("Lint with Extensions: %v", err)
	}
}

// The bundle is the engine's standard list, less the footnote rewrite that
// 0.1 never bundled.
func TestBundleIsTheStandardList(t *testing.T) {
	got := strings.Join(bundleExtensions(), ",")
	for _, name := range carve.StandardExtensions {
		in := strings.Contains(","+got+",", ","+name+",")
		if want := name != "carve.inlineFootnotesToParentheses"; in != want {
			t.Errorf("%s in the bundle: %v, want %v", name, in, want)
		}
	}
}

// Leave one out, in 0.2 names: without carve.spoiler the same document comes
// back unenabled.
func TestTheBundleIsWhatEnablesTheSpoiler(t *testing.T) {
	var without []string
	for _, k := range bundleExtensionKeys {
		if k != "carve.spoiler" {
			without = append(without, k)
		}
	}
	got, err := carve.ToHTMLOptions(spoilerSource, carve.Options{Extensions: without})
	if err != nil {
		t.Fatalf("ToHTMLOptions: %v", err)
	}
	if !strings.Contains(got, spoilerOff) {
		t.Errorf("dropping carve.spoiler from the bundle changed nothing:\n%s", got)
	}
}

// Code callouts render through -extensions with their standard renderer.
func TestExtensionsRenderCodeCallouts(t *testing.T) {
	src := "```go\nx := 1 // <1>\n```\n\n<1> assign x\n"
	res, err := ConvertWithOptions(src, Options{Extensions: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, `class="callout" data-callout="1"`) || !strings.Contains(res.BodyHTML, `class="callouts"`) {
		t.Errorf("callouts did not render:\n%s", res.BodyHTML)
	}
}

// The graphviz preset claims a dot fence too, as in 0.1.
func TestExtensionsClaimTheDotFence(t *testing.T) {
	res, err := ConvertWithOptions("``` dot\ndigraph{a->b}\n```\n", Options{Extensions: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, `<pre class="graphviz"`) {
		t.Errorf("the dot fence was not claimed:\n%s", res.BodyHTML)
	}
}

// The 0.1 bundle carried the navigation extensions; the 0.2 one does again.
func TestExtensionsRenderTheNavigationExtensions(t *testing.T) {
	src := "# Intro\n\n::: toc\n:::\n\n## Part\n\nSee https://example.org and [x](https://ext.org).\n"
	res, err := ConvertWithOptions(src, Options{Extensions: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	for _, want := range []string{
		`class="permalink"`,
		`<span class="section-number">1.1</span>`,
		`<nav class="toc"`,
		`<a href="https://example.org">https://example.org</a>`,
	} {
		if !strings.Contains(res.BodyHTML, want) {
			t.Errorf("-extensions output lacks %s:\n%s", want, res.BodyHTML)
		}
	}
	plain, err := ConvertWithOptions(src, Options{})
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if strings.Contains(plain.BodyHTML, `class="permalink"`) {
		t.Errorf("a navigation extension rendered without -extensions:\n%s", plain.BodyHTML)
	}
}

// External links next to an autolink: carve.autolink runs first in the
// bundle, and the engine then fails the external-links transform.
func TestExtensionsMarkExternalLinks(t *testing.T) {
	t.Skip("carve.externalLinks fails on a view with an autolink node (markup-carve/carve-rs#4484)")
	res, err := ConvertWithOptions("[x](https://ext.org) and https://a.org\n", Options{Extensions: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, `target="_blank"`) {
		t.Errorf("external links are not marked:\n%s", res.BodyHTML)
	}
}
