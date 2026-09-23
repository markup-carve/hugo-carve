# hugo-carve

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

The [design reference](docs/reference.md#honest-limitation-this-is-a-preprocessor-not-a-hugo-plugin)
compares this approach with render hooks, external helpers, modules, and
content adapters.

## Install

```bash
go install github.com/markup-carve/hugo-carve/cmd/hugo-carve@latest
```

`hugo-carve` is installed under `$(go env GOPATH)/bin`. Hugo itself is also
required.

## Build a site

```bash
hugo-carve --content content
hugo
```

Use `--out` to keep generated HTML outside the authored content tree. Other
content and page resources must then be copied or mounted into that tree.
`--clean` removes generated pages. For local preview, rerun the converter when
a Carve source or include changes.

## Includes and rendering policy

Includes are off unless `--includes` is set. Paths are confined to the content
directory by default; `--include-root` selects a different containment root.
`--deps` writes the dependency graph for watch tooling.

Use `--safe` for content that the site does not control. It escapes raw HTML.
Profiles add construct and size limits:

```bash
hugo-carve --content content --profile article
```

An over-limit page stops the build and produces no output file.

`--extensions` enables the bundled extension set. `--static` also enables the
set, then flattens interactive constructs and degrades diagrams and math to
source so the generated HTML has no runtime dependency.

Symbol files and repeated `--symbol NAME=VALUE` flags configure `:name:`
shortcodes. Symbol values are trusted output and must not come from untrusted
input.

## Hugo configuration

Hugo must pass generated HTML through unchanged:

```toml
[security]
allowContent = ['^text/html$']

ignoreFiles = ['\.crv$']
```

The first setting permits generated HTML content pages. The second keeps raw
Carve sources out of `public/`. Use `hugo-carve --safe` to control raw HTML
authored in Carve.

Load Carve's stylesheet and wrap content with `.carve-content` when using
styled extensions. See [Styling](docs/reference.md#styling).

## Reference

The [complete reference](docs/reference.md) documents every CLI flag, includes,
profiles, size caps, symbols, required Hugo settings, and the example site.

## Development

Contributor setup, tests, and maintenance commands are in the
[development guide](docs/development.md).
