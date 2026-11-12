package convert

import (
	"strings"
	"testing"
)

// The opener `carve fmt` writes (CARVE-P11-011) and its lenient spellings
// reach Hugo as the delimiters Hugo reads (hugo-carve#55).
func TestConvert_TypedFrontMatterOpenerReachesHugo(t *testing.T) {
	cases := []struct {
		name, source, wantFM string
	}{
		{"yaml", "---yaml\ntitle: T\ndate: 2026-06-02\n---\n\n# T\n", "---\ntitle: T\ndate: 2026-06-02\n---\n"},
		{"yaml with one space", "--- yaml\ntitle: T\n---\n\n# T\n", "---\ntitle: T\n---\n"},
		{"bare", "---\ntitle: T\n---\n\n# T\n", "---\ntitle: T\n---\n"},
		{"bare with trailing tab", "---\t\ntitle: T\n---  \n\n# T\n", "---\t\ntitle: T\n---  \n"},
		{"uppercase token", "---YAML\ntitle: T\n---\n\n# T\n", "---\ntitle: T\n---\n"},
		{"toml", "---toml\ntitle = \"T\"\n---\n\n# T\n", "+++\ntitle = \"T\"\n+++\n"},
		{"json", "---json\n{\"title\": \"T\"}\n---\n\n# T\n", "{\"title\": \"T\"}"},
		{"crlf", "---yaml\r\ntitle: T\r\n---\r\n\r\n# T\r\n", "---\r\ntitle: T\r\n---\r\n"},
		{"bom", "\uFEFF---yaml\ntitle: T\n---\n\n# T\n", "\uFEFF---\ntitle: T\n---\n"},
		{"empty yaml", "---yaml\n---\n\n# T\n", "---\n---\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Convert(tc.source)
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if res.FrontMatter != tc.wantFM {
				t.Errorf("front matter = %q, want %q", res.FrontMatter, tc.wantFM)
			}
			if !strings.HasPrefix(res.Output, tc.wantFM) {
				t.Errorf("output does not lead with the front matter:\n%s", res.Output)
			}
			if !strings.Contains(res.BodyHTML, "<h1>T</h1>") {
				t.Errorf("body lost: %q", res.BodyHTML)
			}
			again, err := Convert(res.Output)
			if err != nil || again.FrontMatter != res.FrontMatter {
				t.Errorf("second pass changed the front matter: %q, %v", again.FrontMatter, err)
			}
		})
	}
}

// Lines the grammar does not read as an opener stay body, as before.
func TestConvert_NonOpenerDashLinesAreBody(t *testing.T) {
	for _, source := range []string{
		"---  yaml\ntitle: T\n---\n\nbody\n",
		"---\tyaml\ntitle: T\n---\n\nbody\n",
		"---ya-ml\ntitle: T\n---\n\nbody\n",
		"----\ntitle: T\n---\n\nbody\n",
		"---yaml\ntitle: T\n\nbody\n",
	} {
		res, err := Convert(source)
		if err != nil {
			t.Fatalf("Convert(%q): %v", source, err)
		}
		if res.FrontMatter != "" {
			t.Errorf("Convert(%q) took front matter %q", source, res.FrontMatter)
		}
	}
}

// A format Hugo cannot read stops the build instead of dropping the block.
func TestConvert_FrontMatterFormatHugoCannotReadIsAnError(t *testing.T) {
	for source, want := range map[string]string{
		"---neon\ntitle: T\n---\n\nbody\n": `front matter format "neon" has no Hugo equivalent`,
		"---json\n[1, 2]\n---\n\nbody\n":   "not a JSON object",
	} {
		_, err := Convert(source)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Convert(%q) error = %v, want %q", source, err, want)
		}
	}
}
