# hugo-carve guide

## Background

A preprocessor for authoring [Hugo](https://gohugo.io) pages in the
[Carve](https://github.com/markup-carve) markup language.

## Why it is a preprocessor

Hugo has no public API for registering a third-party markup language. Its
content renderers and external helpers are fixed, so a module cannot add
`.crv` as a native page type.

`hugo-carve` runs before Hugo. It converts each Carve page to an HTML content
page, preserves TOML, YAML, or JSON frontmatter, and leaves unrelated HTML
files alone:

```text
foo.crv -> hugo-carve -> foo.html -> hugo -> public/foo/index.html
```

The [design reference](reference.md#honest-limitation-this-is-a-preprocessor-not-a-hugo-plugin)
compares this approach with render hooks, external helpers, modules, and
content adapters.

## Installation notes

`hugo-carve` is installed under `$(go env GOPATH)/bin`. Hugo itself is also
required.

## Build a site


Use `--out` to keep generated HTML outside the authored content tree. Other
content and page resources must then be copied or mounted into that tree.
`--clean` removes generated pages. For local preview, rerun the converter when
a Carve source or include changes.

## Includes and rendering policy

Includes are off unless `--includes` is set. Paths are confined to the content
directory by default; `--include-root` selects a different containment root.
`{{ file.crv @region:name }}` takes one marked region of a file, and
`@rows` and `@cols` take records and columns of a CSV or TSV table include.
`--deps` writes the dependency graph for watch tooling. The pinned Carve 0.2
engine expands includes under that containment root (C2-N-001).

Raw HTML is escaped by default since Carve 0.2. `--raw-html` emits it verbatim;
use it only for pages the site authored itself. Profiles add construct and size
limits:

```bash
hugo-carve --content content --profile article
```

An over-limit page stops the build and produces no output file.

`--extensions` enables the bundled extension set: every Carve 0.2 standard
extension carve-go lists (`carve.StandardExtensions`, less
`carve.inlineFootnotesToParentheses`, and less `carve.cite`, whose `:cite`
citations `carve.references` renders), each with its standard renderer. As in
0.1 that includes the navigation extensions: heading permalinks, heading
numbers, heading level shift, the table of contents (`carve.toc` and the
`::: toc` placement `carve.tocPlacement`), external links, autolink and the
link policy. The 0.2 lint pass reads no extensions (C2-W-109), so `--lint --extensions` lints as
`--lint` does. `--static` renders in the engine's static mode, which
flattens interactive constructs so the generated HTML has no runtime
dependency.

Symbol files and repeated `--symbol NAME=VALUE` flags configure `:name:`
shortcodes. Symbol values are trusted output and must not come from untrusted
input.

## Lint

`--lint` prints the engine's lint findings for every page on stderr, one line
each, and still converts the pages. `--lint-strict` does the same and exits 1
when any page has a finding, which suits CI:

```bash
hugo-carve --content content --lint-strict
```

```
hugo-carve: content/post.crv:3:1: unattached-block-attribute: This block attribute reaches no block: ...
```

With `--includes`, the linter also checks every included Carve file and names
the file of each finding there.

The engine's default rules include hints for non-canonical spellings, such as
a `---` thematic break where Carve writes `***`, and a caption-looking
paragraph next to a table or image. `--lint-strict` fails on these hints as
well. `--caption-hints` only implies `--lint` since Carve 0.2.

Aligned table formatting is not exposed: hugo-carve renders HTML and does not
format Carve.

## Hugo configuration

Hugo must pass generated HTML through unchanged:

```toml
ignoreFiles = ['\.crv$']

[security]
allowContent = ['^text/html$']
```

`ignoreFiles` keeps raw Carve sources out of `public/`, and `allowContent`
permits generated HTML content pages. Keep `ignoreFiles` above `[security]`:
below it, TOML reads it as `security.ignoreFiles` and Hugo ignores it. Raw HTML
authored in Carve is escaped unless you run `hugo-carve --raw-html`.

Load Carve's stylesheet and wrap content with `.carve-content` when using
styled extensions. See [Styling](reference.md#styling).
