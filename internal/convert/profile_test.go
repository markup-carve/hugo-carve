package convert

import (
	"strconv"
	"strings"
	"testing"
)

// The profile option is the one engine option whose failure mode is SILENCE:
// the engine embedded in the pinned carve-go answers an over-cap document with
// an empty render, exit 0 and an empty stderr (markup-carve/carve-rs#1190), so
// a misconfigured build publishes blank pages and nothing says why. Every
// assertion below is therefore on an ERROR or on rendered content - never on
// emptiness alone, which is the bug's own symptom and would stay green with the
// bug present.

// pageOf builds a body of exactly n bytes under a fixed front matter block, so
// a case can sit one byte either side of a cap and say which side it is on.
func pageOf(n int) string {
	return "+++\ntitle = \"P\"\n+++\n" + strings.Repeat("a", n-1) + "\n"
}

// TestConvert_ProfileReachesTheEngine is the plumbing statement: the field is
// forwarded, and the engine's own per-profile behavior shows up in the HTML.
// Without the forward every case here renders identically.
func TestConvert_ProfileReachesTheEngine(t *testing.T) {
	src := "A [link](https://example.com/) and an image ![a](x.png).\n"
	for _, tc := range []struct {
		profile  string
		want     []string
		dontWant []string
	}{
		{"full", []string{`<a href="https://example.com/">link</a>`, `<img src="x.png"`}, nil},
		{"article", []string{`<a href="https://example.com/">link</a>`, `<img src="x.png"`}, nil},
		{"comment", []string{`rel="nofollow ugc"`, "[img: a]"}, []string{"<img"}},
		{"minimal", []string{"[img: a]"}, []string{"<img", "<a href"}},
	} {
		res, err := ConvertWithOptions(src, Options{Profile: tc.profile})
		if err != nil {
			t.Fatalf("profile %q: ConvertWithOptions error: %v", tc.profile, err)
		}
		for _, want := range tc.want {
			if !strings.Contains(res.BodyHTML, want) {
				t.Errorf("profile %q: expected %q in:\n%s", tc.profile, want, res.BodyHTML)
			}
		}
		for _, dont := range tc.dontWant {
			if strings.Contains(res.BodyHTML, dont) {
				t.Errorf("profile %q: did not expect %q in:\n%s", tc.profile, dont, res.BodyHTML)
			}
		}
	}
}

// TestConvert_NoProfileIsUnchanged pins the default: the empty string leaves the
// option off, so an existing site renders exactly as it did before the field
// existed.
func TestConvert_NoProfileIsUnchanged(t *testing.T) {
	src := "A [link](https://example.com/) and an image ![a](x.png).\n"
	withField, err := ConvertWithOptions(src, Options{Profile: ""})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	plain, err := Convert(src)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if withField.Output != plain.Output {
		t.Errorf("an empty Profile changed the output:\n got %q\nwant %q", withField.Output, plain.Output)
	}
	if !strings.Contains(plain.BodyHTML, `<a href="https://example.com/">link</a>`) {
		t.Errorf("the default should not restrict anything, got:\n%s", plain.BodyHTML)
	}
}

// TestConvert_OverCapBodyIsAnError is the ruling on hugo-carve#20: an over-cap
// body is a refusal that names the cap and the actual size, never a blank page
// reported as success.
func TestConvert_OverCapBodyIsAnError(t *testing.T) {
	for _, tc := range []struct {
		profile string
		max     int
	}{
		{"comment", 100_000},
		{"minimal", 10_000},
	} {
		body := tc.max + 1
		res, err := ConvertWithOptions(pageOf(body), Options{Profile: tc.profile})
		if err == nil {
			t.Fatalf("profile %q: a %d-byte body over the %d-byte cap converted without error (Output %q)",
				tc.profile, body, tc.max, res.Output)
		}
		msg := err.Error()
		for _, want := range []string{
			tc.profile,
			strconv.Itoa(body),
			strconv.Itoa(tc.max),
		} {
			if !strings.Contains(msg, want) {
				t.Errorf("profile %q: the refusal does not name %q: %v", tc.profile, want, err)
			}
		}
		if res.Output != "" {
			t.Errorf("profile %q: a refused conversion still produced %d bytes of output", tc.profile, len(res.Output))
		}
	}
}

// TestConvert_AtCapStillRenders is the near miss, and it is what keeps the test
// above honest: a body of exactly the cap renders and returns no error, so a
// build that had simply stopped rendering could not pass both.
func TestConvert_AtCapStillRenders(t *testing.T) {
	for _, tc := range []struct {
		profile string
		max     int
	}{
		{"comment", 100_000},
		{"minimal", 10_000},
	} {
		res, err := ConvertWithOptions(pageOf(tc.max), Options{Profile: tc.profile})
		if err != nil {
			t.Fatalf("profile %q: a body of exactly %d bytes was refused: %v", tc.profile, tc.max, err)
		}
		if !strings.Contains(res.BodyHTML, "<p>aaa") {
			t.Errorf("profile %q: a body at the cap rendered nothing recognizable:\n%.80s", tc.profile, res.BodyHTML)
		}
	}
}

// TestConvert_UncappedProfilesTakeAnyLength pins that "full" and "article" cap
// nothing, so the guard cannot spread to the two profiles a Hugo PAGE actually
// wants.
func TestConvert_UncappedProfilesTakeAnyLength(t *testing.T) {
	for _, profile := range []string{"", "full", "article"} {
		res, err := ConvertWithOptions(pageOf(200_000), Options{Profile: profile})
		if err != nil {
			t.Fatalf("profile %q: a 200000-byte body was refused: %v", profile, err)
		}
		if !strings.Contains(res.BodyHTML, "<p>aaa") {
			t.Errorf("profile %q: rendered nothing recognizable:\n%.80s", profile, res.BodyHTML)
		}
	}
}

// TestConvert_CommentOnlyBodyIsNotRefused is the false positive a guard written
// on emptiness ALONE would produce. A body that is only a `%% ... %%` comment
// renders to nothing with or without a profile, and that is correct output, not
// a discarded document - the guard must not touch it.
func TestConvert_CommentOnlyBodyIsNotRefused(t *testing.T) {
	src := "+++\ntitle = \"C\"\n+++\n\n%% nothing to see %%\n"
	for _, profile := range []string{"", "comment", "minimal"} {
		res, err := ConvertWithOptions(src, Options{Profile: profile})
		if err != nil {
			t.Fatalf("profile %q: a comment-only body was refused: %v", profile, err)
		}
		if !strings.HasPrefix(res.Output, "+++\n") {
			t.Errorf("profile %q: front matter should still lead the page, got %q", profile, res.Output)
		}
	}
}

// TestConvert_FrontMatterDoesNotCountTowardTheCap pins which bytes the cap is
// about. Only the body reaches the engine, so a page whose FILE is over the cap
// while its body is under it has to render - and the refusal message says so.
func TestConvert_FrontMatterDoesNotCountTowardTheCap(t *testing.T) {
	fm := "+++\ntitle = \"P\"\npadding = \"" + strings.Repeat("x", 2_000) + "\"\n+++\n"
	src := fm + strings.Repeat("a", 9_998) + "\n"
	if len(src) <= 10_000 {
		t.Fatalf("fixture is not over the cap as a whole file: %d bytes", len(src))
	}
	res, err := ConvertWithOptions(src, Options{Profile: "minimal"})
	if err != nil {
		t.Fatalf("a %d-byte file with a 9999-byte body was refused: %v", len(src), err)
	}
	if !strings.Contains(res.BodyHTML, "<p>aaa") {
		t.Errorf("rendered nothing recognizable:\n%.80s", res.BodyHTML)
	}
}

// TestConvert_UnknownProfileSurfacesTheEngineError pins that the name itself is
// still the engine's business: this package forwards it and reports what comes
// back, rather than keeping a second list of valid names to disagree with.
func TestConvert_UnknownProfileSurfacesTheEngineError(t *testing.T) {
	for _, name := range []string{"nope", "COMMENT"} {
		_, err := ConvertWithOptions("hi\n", Options{Profile: name})
		if err == nil {
			t.Fatalf("profile %q: expected the engine to refuse an unknown name", name)
		}
		if !strings.Contains(err.Error(), "unknown profile") {
			t.Errorf("profile %q: expected the engine's own message, got: %v", name, err)
		}
	}
}

// TestCheckProfileCap_DefersToARenderThatSucceeded is the cap-drift case, and it
// is the reason the guard tests emptiness at all. profileMaxBytes restates a
// number that lives in the engine; if the engine RAISES a cap, an over-cap body
// by this package's stale reckoning still renders, and a guard written on
// length alone would refuse a document the engine was perfectly happy with.
func TestCheckProfileCap_DefersToARenderThatSucceeded(t *testing.T) {
	body := strings.Repeat("a", 10_001)
	if err := checkProfileCap("minimal", body, "<p>rendered</p>\n"); err != nil {
		t.Errorf("a body the engine rendered should never be refused here, got: %v", err)
	}
	if err := checkProfileCap("minimal", body, "\n"); err == nil {
		t.Error("a body the engine discarded should be refused")
	}
}

// TestProfileMaxBytes_CoversOnlyTheCappedProfiles keeps the table from growing a
// number for a profile that caps nothing, which would refuse pages the engine
// accepts.
func TestProfileMaxBytes_CoversOnlyTheCappedProfiles(t *testing.T) {
	want := map[string]int{"comment": 100_000, "minimal": 10_000}
	if len(profileMaxBytes) != len(want) {
		t.Fatalf("profileMaxBytes = %v, want %v", profileMaxBytes, want)
	}
	for name, max := range want {
		if profileMaxBytes[name] != max {
			t.Errorf("%s: cap is %d, want %d", name, profileMaxBytes[name], max)
		}
	}
}

// TestConvert_RawHTMLPerProfile pins the row of the README table that is easiest
// to get backwards: "full" is the engine's FULL behavior, so it emits raw HTML
// exactly as no profile does, and only the three restricting profiles escape it.
// Documenting the opposite would tell a site it was hardened when it was not.
func TestConvert_RawHTMLPerProfile(t *testing.T) {
	src := "```=html\n<div class=\"x\">raw</div>\n```\n"
	const raw = `<div class="x">raw</div>`

	for _, profile := range []string{"", "full"} {
		res, err := ConvertWithOptions(src, Options{Profile: profile})
		if err != nil {
			t.Fatalf("profile %q: ConvertWithOptions error: %v", profile, err)
		}
		if !strings.Contains(res.BodyHTML, raw) {
			t.Errorf("profile %q should emit raw HTML unchanged, got:\n%s", profile, res.BodyHTML)
		}
	}
	for _, profile := range []string{"article", "comment", "minimal"} {
		res, err := ConvertWithOptions(src, Options{Profile: profile})
		if err != nil {
			t.Fatalf("profile %q: ConvertWithOptions error: %v", profile, err)
		}
		if strings.Contains(res.BodyHTML, raw) {
			t.Errorf("profile %q should not emit a live raw block, got:\n%s", profile, res.BodyHTML)
		}
		if !strings.Contains(res.BodyHTML, "&lt;div") {
			t.Errorf("profile %q should escape the raw block, got:\n%s", profile, res.BodyHTML)
		}
	}
}

// TestConvert_FullMatchesNoProfile pins the other half of the same row: naming
// "full" explicitly is a statement of intent, not a change in output.
func TestConvert_FullMatchesNoProfile(t *testing.T) {
	src := "# H\n\nA [link](https://example.com/), ![a](x.png) and *bold*.\n"
	full, err := ConvertWithOptions(src, Options{Profile: "full"})
	if err != nil {
		t.Fatalf("ConvertWithOptions error: %v", err)
	}
	none, err := Convert(src)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if full.Output != none.Output {
		t.Errorf("--profile full should render what no profile renders:\n full %q\n none %q", full.Output, none.Output)
	}
}
