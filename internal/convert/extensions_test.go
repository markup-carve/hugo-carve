package convert

import (
	"sort"
	"strings"
	"testing"

	carve "github.com/markup-carve/carve-go"
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
	res, err := ConvertWithOptions(spoilerSource, Options{Static: true})
	if err != nil {
		t.Fatalf("ConvertWithOptions: %v", err)
	}
	if !strings.Contains(res.BodyHTML, spoilerOn) {
		t.Errorf("-static no longer enables the bundle:\n%s", res.BodyHTML)
	}
}
