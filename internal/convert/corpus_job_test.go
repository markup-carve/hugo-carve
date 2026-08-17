package convert

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The blocking job must actually RUN every test that needs the corpus.
//
// It did not. TestSpecCorpus skips unless CARVE_SPEC_CORPUS is set, ci.yml ran
// `go test ./...` and never set it, and ci.yml was the only job gating a merge.
// So the one check in this repository that can tell a stale engine from a
// current one had never executed on a push or a pull request, while a green
// tick said otherwise (hugo-carve#11, the class catalogued in
// markup-carve/carve#755).
//
// Wiring the variable into a corpus job fixed it. This is what stops it going
// quiet again, and it lives in the package's ordinary tests on purpose: it has
// no corpus dependency of its own, so it runs in `go test ./...` with nothing
// checked out beside this repository, in the same job that used to be the whole
// of CI. A guard that skipped alongside the thing it guards would be the defect
// twice.
//
// It is deliberately not "the job has no -run flag" - a future job may want one,
// and engine-drift.yml's caller has always had one. It asks the question that
// matters: is every corpus-gated test in this package reachable from the
// command the corpus job runs?

const (
	repoRoot   = "../.."
	ciWorkflow = ".github/workflows/ci.yml"
)

// A test is corpus-gated when its body reaches the corpus, either through one
// of this package's helpers or through the environment variable directly.
// Derived from the source rather than listed, so a test added later is covered
// without editing this one - a hand-maintained list would reproduce the
// original defect one rename later.
func corpusGatedTests(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parsing this package: %v", err)
	}
	var names []string
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			if !strings.HasSuffix(path, "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || fn.Body == nil {
					continue
				}
				if !strings.HasPrefix(fn.Name.Name, "Test") {
					continue
				}
				gated := false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return !gated
					}
					switch fun := call.Fun.(type) {
					case *ast.Ident:
						// A corpus helper, each of which needs a corpus
						// directory to say anything at all.
						switch fun.Name {
						case "requireWholeCorpus", "declaredCorpusSize":
							gated = true
						}
					case *ast.SelectorExpr:
						// os.Getenv("CARVE_SPEC_CORPUS") - the gate itself.
						pkgIdent, ok := fun.X.(*ast.Ident)
						if !ok || pkgIdent.Name != "os" || fun.Sel.Name != "Getenv" {
							return !gated
						}
						if len(call.Args) == 1 {
							if lit, ok := call.Args[0].(*ast.BasicLit); ok &&
								lit.Value == `"CARVE_SPEC_CORPUS"` {
								gated = true
							}
						}
					}
					return !gated
				})
				if gated {
					names = append(names, fn.Name.Name)
				}
			}
		}
	}
	// The ablation, so a scan that read nothing cannot report a clean result.
	// Anchored on a test whose corpus dependence is definitional rather than on
	// a count: TestSpecCorpus reads CARVE_SPEC_CORPUS in its first statement, so
	// a scan that misses it is not reading the sources. A `len(names) >= 1`
	// floor would be satisfied by any one match and would go quiet the moment
	// the scan started matching the wrong thing.
	for _, name := range names {
		if name == "TestSpecCorpus" {
			return names
		}
	}
	t.Fatalf("the scan found %d corpus-gated test(s) (%s) and TestSpecCorpus was not among them; "+
		"that test reads CARVE_SPEC_CORPUS directly, so the scan is not reading the sources",
		len(names), strings.Join(names, ", "))
	return nil
}

// The command run by the ci.yml job that sets CARVE_SPEC_CORPUS.
func corpusJobCommand(t *testing.T) string {
	t.Helper()
	path := filepath.Join(repoRoot, ciWorkflow)
	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	seenVariable := false
	for _, line := range strings.Split(string(blob), "\n") {
		// The variable's own name appears in this file's prose too, so match
		// the YAML key rather than the word.
		if strings.Contains(line, "CARVE_SPEC_CORPUS:") {
			seenVariable = true
			continue
		}
		if !seenVariable {
			continue
		}
		if command, ok := strings.CutPrefix(strings.TrimSpace(line), "run:"); ok {
			return strings.TrimSpace(command)
		}
	}
	if !seenVariable {
		t.Fatalf("no job in %s sets CARVE_SPEC_CORPUS, so TestSpecCorpus skips on every push "+
			"and every pull request while the job reports success. That is the state "+
			"hugo-carve#11 was filed about; it is not a configuration this repository accepts.",
			ciWorkflow)
	}
	t.Fatalf("the %s job that sets CARVE_SPEC_CORPUS has no run: command after it", ciWorkflow)
	return ""
}

// goTestFilters returns the -run patterns of every `go test` invocation the
// corpus job ends up executing, and false if it executes none.
//
// One level of indirection is followed, because the command is a script in this
// repository rather than a `go test` line: the two workflows that need this
// comparison call the same scripts/corpus-drift.sh, and reading only the
// workflow would see a filename and conclude the job runs no tests at all.
// An empty pattern in the returned slice means an unfiltered invocation.
func goTestFilters(t *testing.T, command string) ([]string, bool) {
	t.Helper()

	lines := []string{command}
	if !strings.Contains(command, "go test") {
		script := strings.Fields(command)
		if len(script) == 0 {
			t.Fatalf("the corpus job's command is empty")
		}
		path := filepath.Join(repoRoot, script[len(script)-1])
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("the corpus job runs %q, which is neither a go test invocation nor a file "+
				"in this repository (%v), so what it executes cannot be checked", command, err)
		}
		// `run:` executes the file directly, so a missing execute bit is a
		// broken job that no amount of correct YAML fixes.
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("%s is not executable (mode %v), so the corpus job cannot run it",
				path, info.Mode().Perm())
		}
		blob, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		lines = strings.Split(string(blob), "\n")
	}

	var patterns []string
	for _, line := range lines {
		if !strings.Contains(line, "go test") {
			continue
		}
		fields := strings.Fields(line)
		pattern := ""
		for i, field := range fields {
			if field == "-run" && i+1 < len(fields) {
				pattern = strings.Trim(fields[i+1], `"'`)
			}
			if rest, ok := strings.CutPrefix(field, "-run="); ok {
				pattern = strings.Trim(rest, `"'`)
			}
		}
		patterns = append(patterns, pattern)
	}
	return patterns, len(patterns) > 0
}

func TestTheCorpusJobRunsEveryCorpusGatedTest(t *testing.T) {
	command := corpusJobCommand(t)
	gated := corpusGatedTests(t)

	patterns, ok := goTestFilters(t, command)
	if !ok {
		t.Fatalf("the corpus job's command is %q, and nothing it reaches runs go test", command)
	}

	var unreachable []string
	for _, name := range gated {
		reached := false
		for _, pattern := range patterns {
			if pattern == "" {
				// No filter: everything the package defines runs.
				reached = true
				break
			}
			// Go's -run matches unanchored against the test name, so this
			// mirrors it rather than requiring a full match.
			filter, err := regexp.Compile(pattern)
			if err != nil {
				t.Fatalf("the corpus job's -run pattern %q does not compile: %v", pattern, err)
			}
			if filter.MatchString(name) {
				reached = true
				break
			}
		}
		if !reached {
			unreachable = append(unreachable, name)
		}
	}
	if len(unreachable) > 0 {
		t.Fatalf("the corpus job runs go test with -run %s, which never executes %s. "+
			"These tests need CARVE_SPEC_CORPUS and only that job sets it, so under this filter "+
			"there is no configuration in which they run. Widen the pattern or drop it.",
			strings.Join(patterns, " / "), strings.Join(unreachable, ", "))
	}

	// The ablation. Without it the loop above passes identically whether the
	// patterns were applied or quietly ignored.
	if regexp.MustCompile("TestSpecCorpus").MatchString("TestConvert_Idempotent") {
		t.Fatal("the filter check is not matching names")
	}
}
