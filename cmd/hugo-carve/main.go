// Command hugo-carve is a preprocessor that converts Carve content files
// (*.crv) into Hugo-consumable HTML content pages.
//
// Hugo exposes no public plugin API for registering a third-party markup
// language, so hugo-carve runs BEFORE `hugo`: it renders each Carve file's
// body to HTML (via carve-go) while preserving the file's front matter, and
// writes a `.html` page that Hugo serves as passthrough content. The typical
// workflow is:
//
//	hugo-carve --content content   # produces content/*.html from *.crv
//	hugo                           # builds the site as usual
//
// The tool is idempotent: re-running it reconverts from the Carve sources and
// rewrites the HTML outputs; HTML files without a Carve source are left alone.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/markup-carve/hugo-carve/internal/convert"
)

// repeatable collects a flag that may be given more than once, in the order
// the flags were written. The order is load-bearing for the symbol sources:
// they merge left to right.
type repeatable []string

func (r *repeatable) String() string { return strings.Join(*r, ",") }

func (r *repeatable) Set(v string) error {
	*r = append(*r, v)
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "hugo-carve:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("hugo-carve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	contentDir := fs.String("content", "content", "content directory to scan for Carve files")
	outDir := fs.String("out", "", "output directory (default: in place, next to the source)")
	clean := fs.Bool("clean", false, "remove generated .html outputs instead of building them")
	quiet := fs.Bool("quiet", false, "suppress per-file log output")
	extensions := fs.Bool("extensions", false, "enable the bundled extensions (diagram presets - mermaid, plantuml, d2, graphviz, ... - plus details, spoiler, code-callouts, color, math)")
	static := fs.Bool("static", false, "self-contained static HTML: flatten interactive constructs and degrade diagrams/math to source (implies -extensions)")
	safe := fs.Bool("safe", false, "escape raw HTML (=html blocks and {=html} spans) instead of emitting it; set this for content the site did not author")
	var symbolFiles repeatable
	fs.Var(&symbolFiles, "symbols", "path to a JSON `file` mapping a symbol name to what :name: renders as (repeatable; merged left to right)")
	var symbolPairs repeatable
	fs.Var(&symbolPairs, "symbol", "one symbol as `NAME=VALUE` (repeatable; applied after -symbols, so it overrides a file)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: hugo-carve [flags]\n\n")
		fmt.Fprintf(stderr, "Converts *.crv files into Hugo HTML content pages.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	// This command takes flags and nothing else, so an operand is always a
	// mistake - and a silent one, because Go's flag package STOPS parsing at
	// the first non-flag argument. `hugo-carve content --safe` would otherwise
	// read "content" as a stray operand, never apply --safe, and exit 0 having
	// passed raw HTML straight through. A security switch that a typo can
	// disable without a word is worse than no switch, so say so instead.
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q: hugo-carve takes flags only (did you mean --content %s?); any flag after it was ignored", fs.Arg(0), fs.Arg(0))
	}

	symbols, err := symbolMap(symbolFiles, symbolPairs)
	if err != nil {
		return err
	}

	c := &converter{
		contentDir: *contentDir,
		outDir:     *outDir,
		clean:      *clean,
		quiet:      *quiet,
		extensions: *extensions,
		static:     *static,
		safe:       *safe,
		symbols:    symbols,
		log:        stdout,
	}
	return c.walk()
}

type converter struct {
	contentDir string
	outDir     string
	clean      bool
	quiet      bool
	extensions bool
	static     bool
	safe       bool
	symbols    map[string]string
	log        *os.File
}

// symbolMap merges the symbol sources into the single map handed to the
// engine: every -symbols file in the order given, then every -symbol pair, so
// a generated map can carry a handful of site-specific overrides.
//
// With no sources at all it returns nil rather than an empty map, so the
// default path hands the engine exactly what it handed it before this flag
// existed.
//
// Only the shape of the SOURCE is checked here - a file has to be a JSON
// object of strings, and a pair has to contain "=" so a name and a value can
// be told apart at all. What a name and a value may CONTAIN is the engine's
// contract, and carve-go refuses an entry it cannot pass through intact with
// its own message; re-stating those rules here would only give a site author
// two chances to read a different one.
func symbolMap(files, pairs []string) (map[string]string, error) {
	if len(files) == 0 && len(pairs) == 0 {
		return nil, nil
	}
	symbols := map[string]string{}
	for _, path := range files {
		loaded, err := loadSymbolFile(path)
		if err != nil {
			return nil, err
		}
		for name, value := range loaded {
			symbols[name] = value
		}
	}
	for _, pair := range pairs {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("-symbol %q: expected NAME=VALUE", pair)
		}
		symbols[name] = value
	}
	return symbols, nil
}

// loadSymbolFile reads one JSON object of name -> string.
func loadSymbolFile(path string) (map[string]string, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read symbols file: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(blob, &raw); err != nil {
		return nil, fmt.Errorf("symbols file %q: expected a JSON object mapping a name to a string: %w", path, err)
	}
	// A bare `null` unmarshals into a nil map WITHOUT an error, so it would
	// otherwise be accepted as an empty map and the site would build with no
	// symbols and no complaint. That is the worst outcome a misconfigured file
	// can have: everything succeeds and every shortcode silently stays literal.
	if raw == nil {
		return nil, fmt.Errorf("symbols file %q: expected a JSON object mapping a name to a string, got null", path)
	}
	symbols := make(map[string]string, len(raw))
	for name, encoded := range raw {
		var value string
		if err := json.Unmarshal(encoded, &value); err != nil {
			return nil, fmt.Errorf("symbols file %q: value for symbol %q must be a string", path, name)
		}
		symbols[name] = value
	}
	return symbols, nil
}

// carveExts are the recognized Carve file extensions.
var carveExts = []string{".crv"}

func isCarve(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range carveExts {
		if ext == e {
			return true
		}
	}
	return false
}

func (c *converter) walk() error {
	info, err := os.Stat(c.contentDir)
	if err != nil {
		return fmt.Errorf("content directory %q: %w", c.contentDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("content path %q is not a directory", c.contentDir)
	}

	count := 0
	err = filepath.WalkDir(c.contentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isCarve(d.Name()) {
			return nil
		}
		if err := c.convertFile(path); err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}
	if !c.quiet {
		action := "converted"
		if c.clean {
			action = "cleaned"
		}
		fmt.Fprintf(c.log, "hugo-carve: %s %d Carve file(s)\n", action, count)
	}
	return nil
}

// outputPath maps a Carve source path to its HTML output path, honoring --out.
func (c *converter) outputPath(src string) (string, error) {
	htmlPath := strings.TrimSuffix(src, filepath.Ext(src)) + ".html"
	if c.outDir == "" {
		return htmlPath, nil
	}
	rel, err := filepath.Rel(c.contentDir, htmlPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(c.outDir, rel), nil
}

func (c *converter) convertFile(src string) error {
	out, err := c.outputPath(src)
	if err != nil {
		return err
	}

	if c.clean {
		if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %q: %w", out, err)
		}
		if !c.quiet {
			fmt.Fprintf(c.log, "  - removed %s\n", out)
		}
		return nil
	}

	srcBytes, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %q: %w", src, err)
	}

	res, err := convert.ConvertWithOptions(string(srcBytes), convert.Options{
		Extensions: c.extensions,
		Static:     c.static,
		Safe:       c.safe,
		Symbols:    c.symbols,
	})
	if err != nil {
		return fmt.Errorf("convert %q: %w", src, err)
	}

	if dir := filepath.Dir(out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %q: %w", dir, err)
		}
	}
	if err := os.WriteFile(out, []byte(res.Output), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", out, err)
	}
	if !c.quiet {
		fmt.Fprintf(c.log, "  - %s -> %s\n", src, out)
	}
	return nil
}
