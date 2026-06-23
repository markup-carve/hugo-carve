# hugo-carve

A preprocessor that lets you author [Hugo](https://gohugo.io) pages in the
[Carve](https://github.com/markup-carve) markup language.

## Honest limitation: this is a preprocessor, not a Hugo plugin

**Hugo has no public plugin API for registering a third-party markup
language.** Unlike Jekyll (which has a `Converter` plugin interface) or Eleventy
(which has `addExtension`), Hugo's content rendering is built in. Its markup
handlers are a fixed, hardcoded set: Goldmark (Markdown), plus a small number of
external helpers (AsciiDoc, Pandoc, reStructuredText, Org). You cannot teach
`hugo` to render a new `.crv` file extension the way you can with those other
generators, and the external-helper mechanism is not user-extensible to add your
own binary for a new extension.

So `hugo-carve` is **not** a drop-in renderer plugin. It is a **preprocessor**:
a small Go CLI that runs *before* `hugo`. It walks your `content/` tree,
converts every `*.crv` / `*.carve` file's body to HTML using
[`carve-go`](https://github.com/markup-carve/carve-go), preserves the file's
front matter verbatim, and writes a sibling `*.html` content page. Hugo then
builds the site normally, reading the front matter for `title` and other params
and serving the rendered HTML as page content.

### Why the other Hugo extension points do not fit

| Mechanism | Why it does not work for Carve |
| --- | --- |
| **Custom markup handler** | No public API exists. Markup handlers are a hardcoded built-in set; you cannot register a `.crv` renderer. |
| **External helpers** (`asciidoctor`, `pandoc`, `rst`) | The list of external converters is hardcoded; users cannot add a binary for a new format/extension. |
| **Render hooks** (`markup.goldmark` render hooks) | Scoped to elements *inside* Goldmark Markdown (links, images, headings, code blocks, tables, etc.). They cannot introduce a new top-level format. |
| **Hugo Modules** | A dependency/asset-sharing mechanism, not a renderer. |
| **Content adapters** (`_content.gotmpl`, Hugo v0.126+) | A viable alternative: a build-time Go template can call out and `AddPage` with `content.value` + `content.mediaType` + front matter. It works, but it pushes per-file I/O and the carve-go call into template logic, is harder to debug, and still emits HTML as the page body. A standalone preprocessor is simpler, testable as plain Go, and keeps the conversion step explicit and idempotent. |

The preprocessor approach was chosen because it is the simplest path that
produces a correct site with current Hugo, is unit-testable as ordinary Go, and
keeps `.crv` as the readable source of truth in your repo.

## How it works

```
foo.crv  ──hugo-carve──▶  foo.html  ──hugo──▶  public/foo/index.html
(front matter + Carve)    (front matter + HTML)   (final page)
```

1. `hugo-carve` splits each `.crv`/`.carve` file into front matter and body.
2. The body is rendered to HTML by the embedded Carve engine (`carve-go`, a
   WASI build of `carve-rs` driven by the pure-Go wazero runtime: no cgo, no
   external binary).
3. The front matter (TOML `+++`, YAML `---`, or JSON `{ ... }`) is preserved
   exactly. Hugo reads it for `title`, `date`, params, etc.
4. The result is written as a `.html` content page that Hugo serves verbatim.

The converter is **idempotent**: it always reconverts from the `.crv` source,
so re-running it produces identical output. HTML files without a matching Carve
source are left untouched.

## Install

```bash
go install github.com/markup-carve/hugo-carve/cmd/hugo-carve@latest
```

This puts `hugo-carve` in `$(go env GOPATH)/bin` (commonly `~/go/bin`; make sure
that is on your `PATH`). You also need [`hugo`](https://gohugo.io) itself.

## Workflow: convert, then build

Run the preprocessor, then run Hugo:

```bash
hugo-carve --content content   # *.crv -> *.html (in place)
hugo                           # build the site as usual
```

Or wrap both in one command (Makefile, npm script, shell alias):

```bash
hugo-carve --content content && hugo
```

For local authoring with live reload, run the converter first, then
`hugo server`. Re-run `hugo-carve` whenever you edit a `.crv` file (or wire it
into your watch tooling).

### CLI

```
Usage: hugo-carve [flags]

Converts *.crv / *.carve files into Hugo HTML content pages.

  -clean
        remove generated .html outputs instead of building them
  -content string
        content directory to scan for Carve files (default "content")
  -out string
        output directory (default: in place, next to the source)
  -quiet
        suppress per-file log output
```

- `--content DIR` selects the tree to scan (default `content`).
- `--out DIR` writes the generated `.html` into a separate build directory,
  mirroring the source layout, instead of next to the `.crv` source. Useful if
  you prefer to keep generated files out of your authored tree.
- `--clean` removes the generated `.html` files (the inverse operation).

## Required Hugo configuration

Hugo denies raw `text/html` page content by default
(`security.allowContent` defaults to `['! ^text/html$']`). Because the
converter produces HTML content pages, you must allow `text/html` in your site
config:

```toml
# hugo.toml
[security]
  allowContent = ['^text/html$']
```

It is also recommended to stop Hugo from copying the raw `.crv` sources into the
built site (they sit next to the generated `.html`):

```toml
ignoreFiles = ['\.crv$', '\.carve$']
```

## Sample

A `.crv` content file with front matter:

```
+++
title = "Carve on Hugo"
date = 2026-06-20
+++

# Welcome to Carve on Hugo

This page was authored in *Carve* and converted to HTML by /hugo-carve/.

- Front matter is preserved.
- The Carve body is rendered to HTML.
```

After `hugo-carve --content content`, the sibling `.html` keeps the front matter
and replaces the body with rendered HTML; `hugo` then produces the final page.
A complete, buildable site lives in [`example/`](./example).

## Example site

```bash
cd example
hugo-carve --content content   # or: go run ../cmd/hugo-carve --content content
hugo
# open example/public/index.html
```

## Development

This repo uses a local `replace` for `carve-go` during development:

```
replace github.com/markup-carve/carve-go => /tmp/go-carve
```

For published use, drop the `replace` and pin a released version:

```
require github.com/markup-carve/carve-go vX.Y.Z
```

Run the tests:

```bash
go build ./...
go vet ./...
go test ./...
```

## License

[MIT](./LICENSE) (c) markup-carve.
