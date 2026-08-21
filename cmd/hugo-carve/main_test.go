package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The symbol map is the one piece of configuration this command assembles
// rather than passes straight through, so it is the one piece that can be
// assembled wrongly. These cases cover the assembly; what a name and a value
// may contain is the engine's contract and is pinned in internal/convert.

// TestSymbolMap_NoSourcesIsNil pins the default. Returning an empty map instead
// of nil would still render the same today, but it would hand the engine an
// argument list where it previously got none, so the distinction is worth
// keeping honest at the boundary.
func TestSymbolMap_NoSourcesIsNil(t *testing.T) {
	got, err := symbolMap(nil, nil)
	if err != nil {
		t.Fatalf("symbolMap error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil with no sources, got %v", got)
	}
}

// TestSymbolMap_PairsAccumulate pins that -symbol may be repeated, and that a
// value may itself contain "=" - the split is at the FIRST one, matching how
// the engine reads the same argument.
func TestSymbolMap_PairsAccumulate(t *testing.T) {
	got, err := symbolMap(nil, []string{"rocket=\U0001F680", "eq=a=b", "empty="})
	if err != nil {
		t.Fatalf("symbolMap error: %v", err)
	}
	want := map[string]string{"rocket": "\U0001F680", "eq": "a=b", "empty": ""}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s: got %q, want %q", name, got[name], value)
		}
	}
}

// TestSymbolMap_PairWithoutSeparatorIsRefused pins the one shape this command
// has to reject itself: without "=" there is no way to tell a name from a
// value, so there is nothing to forward.
func TestSymbolMap_PairWithoutSeparatorIsRefused(t *testing.T) {
	_, err := symbolMap(nil, []string{"rocket"})
	if err == nil {
		t.Fatal("expected an error for a pair with no separator")
	}
	if !strings.Contains(err.Error(), "NAME=VALUE") {
		t.Errorf("error should say what the shape is, got: %v", err)
	}
}

// TestSymbolMap_FilesMergeLeftToRightThenPairs pins the merge order the
// documentation promises: a later file overrides an earlier one, and an inline
// -symbol overrides every file, so a generated map can carry a few
// site-specific overrides.
func TestSymbolMap_FilesMergeLeftToRightThenPairs(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "second.json")
	write(t, first, `{"a": "1", "b": "1", "c": "1"}`)
	write(t, second, `{"b": "2", "c": "2"}`)

	got, err := symbolMap([]string{first, second}, []string{"c=3"})
	if err != nil {
		t.Fatalf("symbolMap error: %v", err)
	}
	want := map[string]string{"a": "1", "b": "2", "c": "3"}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s: got %q, want %q", name, got[name], value)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestSymbolMap_RejectsUnusableFile covers the three ways a file can fail to be
// a map of names to strings. Each has to name the file, because a site can pass
// several.
func TestSymbolMap_RejectsUnusableFile(t *testing.T) {
	dir := t.TempDir()
	notJSON := filepath.Join(dir, "not-json.json")
	write(t, notJSON, "{")
	notObject := filepath.Join(dir, "array.json")
	write(t, notObject, `["a", "b"]`)
	notStrings := filepath.Join(dir, "numbers.json")
	write(t, notStrings, `{"a": "ok", "n": 3}`)

	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{"missing", filepath.Join(dir, "absent.json"), "read symbols file"},
		{"not JSON", notJSON, "expected a JSON object"},
		{"not an object", notObject, "expected a JSON object"},
		{"value not a string", notStrings, `value for symbol "n" must be a string`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := symbolMap([]string{tc.path}, nil)
			if err == nil {
				t.Fatalf("expected an error for %s", tc.path)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected %q in the error, got: %v", tc.want, err)
			}
		})
	}
}

// TestRun_SymbolFlagsReachTheRenderedPage is the end-to-end statement: the
// flags are registered, parsed, merged and handed to the converter, and the
// substitution lands in the file Hugo will read. Every unit above could pass
// with the flags never wired into run at all.
func TestRun_SymbolFlagsReachTheRenderedPage(t *testing.T) {
	dir := t.TempDir()
	content := filepath.Join(dir, "content")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(content, "page.crv"), "+++\ntitle = \"S\"\n+++\n\nShip it :rocket: :logo: :shrug:\n")
	write(t, filepath.Join(dir, "symbols.json"), `{"rocket": "FROM-FILE", "logo": "<img src='/l.svg'>"}`)

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	err = run([]string{
		"--content", content,
		"--symbols", filepath.Join(dir, "symbols.json"),
		"--symbol", "rocket=FROM-FLAG",
		"--quiet",
	}, devNull, devNull)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	blob, err := os.ReadFile(filepath.Join(content, "page.html"))
	if err != nil {
		t.Fatalf("converted page: %v", err)
	}
	out := string(blob)
	if !strings.Contains(out, "FROM-FLAG") {
		t.Errorf("-symbol should override the file, got:\n%s", out)
	}
	if strings.Contains(out, "FROM-FILE") {
		t.Errorf("-symbol did not override the file, got:\n%s", out)
	}
	if !strings.Contains(out, "<img src='/l.svg'>") {
		t.Errorf("a file entry should reach the page, got:\n%s", out)
	}
	if !strings.Contains(out, ":shrug:") {
		t.Errorf("an unmapped name should stay literal, got:\n%s", out)
	}
	if !strings.HasPrefix(out, "+++\n") {
		t.Errorf("front matter should still lead the page, got:\n%s", out)
	}
}

// TestRun_WithoutSymbolFlagsLeavesShortcodeLiteral is the other half: the
// default path is untouched, and a site that configures nothing gets exactly
// what it got before.
func TestRun_WithoutSymbolFlagsLeavesShortcodeLiteral(t *testing.T) {
	dir := t.TempDir()
	content := filepath.Join(dir, "content")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(content, "page.crv"), "Ship it :rocket:\n")

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	if err := run([]string{"--content", content, "--quiet"}, devNull, devNull); err != nil {
		t.Fatalf("run error: %v", err)
	}
	blob, err := os.ReadFile(filepath.Join(content, "page.html"))
	if err != nil {
		t.Fatalf("converted page: %v", err)
	}
	if !strings.Contains(string(blob), ":rocket:") {
		t.Errorf("with no map the shortcode should stay literal, got:\n%s", blob)
	}
}

// TestRun_RefusesAnUnusableSymbolSourceBeforeConverting pins that a bad source
// stops the run rather than converting the tree with a silently shorter map.
func TestRun_RefusesAnUnusableSymbolSourceBeforeConverting(t *testing.T) {
	dir := t.TempDir()
	content := filepath.Join(dir, "content")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(content, "page.crv"), "Ship it :rocket:\n")

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	err = run([]string{"--content", content, "--symbol", "rocket", "--quiet"}, devNull, devNull)
	if err == nil {
		t.Fatal("expected run to refuse a malformed -symbol")
	}
	if _, statErr := os.Stat(filepath.Join(content, "page.html")); statErr == nil {
		t.Error("no page should have been written when the symbol source is unusable")
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
