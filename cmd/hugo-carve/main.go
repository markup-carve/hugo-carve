// Command hugo-carve is a preprocessor that converts Carve content files
// (*.crv / *.carve) into Hugo-consumable HTML content pages.
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
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/markup-carve/hugo-carve/internal/convert"
)

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
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: hugo-carve [flags]\n\n")
		fmt.Fprintf(stderr, "Converts *.crv / *.carve files into Hugo HTML content pages.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	c := &converter{
		contentDir: *contentDir,
		outDir:     *outDir,
		clean:      *clean,
		quiet:      *quiet,
		log:        stdout,
	}
	return c.walk()
}

type converter struct {
	contentDir string
	outDir     string
	clean      bool
	quiet      bool
	log        *os.File
}

// carveExts are the recognized Carve file extensions.
var carveExts = []string{".crv", ".carve"}

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

	res, err := convert.Convert(string(srcBytes))
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
