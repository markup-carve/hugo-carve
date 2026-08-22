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
converts every `*.crv` file's body to HTML using
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

1. `hugo-carve` splits each `.crv` file into front matter and body.
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

Converts *.crv files into Hugo HTML content pages.

  -clean
        remove generated .html outputs instead of building them
  -content string
        content directory to scan for Carve files (default "content")
  -extensions
        enable the bundled extensions (diagram presets - mermaid, plantuml, d2,
        graphviz, ... - plus details, spoiler, code-callouts, color, math)
  -out string
        output directory (default: in place, next to the source)
  -profile full|article|comment|minimal
        engine profile restricting what a document may contain and how large
        it may be: full|article|comment|minimal (default: off, the engine's
        full behavior). comment and minimal also cap the body at 100000 and
        10000 bytes; an over-cap page is an error, never a blank page
  -quiet
        suppress per-file log output
  -safe
        escape raw HTML (=html blocks and {=html} spans) instead of emitting
        it; set this for content the site did not author
  -static
        self-contained static HTML: flatten interactive constructs and degrade
        diagrams/math to source (implies -extensions)
  -symbol NAME=VALUE
        one symbol as NAME=VALUE (repeatable; applied after -symbols, so it
        overrides a file)
  -symbols file
        path to a JSON file mapping a symbol name to what :name: renders as
        (repeatable; merged left to right)
```

- `--content DIR` selects the tree to scan (default `content`).
- `--out DIR` writes the generated `.html` into a separate build directory,
  mirroring the source layout, instead of next to the `.crv` source. Useful if
  you prefer to keep generated files out of your authored tree.
- `--clean` removes the generated `.html` files (the inverse operation).
- `--extensions` enables the engine's bundled extensions. Diagram fences then
  render as hydration elements (`<pre class="plantuml">`, `<pre class="mermaid">`,
  ...) that a client-side or build-step renderer turns into pictures - e.g.
  Graphviz/D2 offline via [`@markup-carve/carve-grammars`](https://github.com/markup-carve/carve-grammars)
  WASM helpers, PlantUML via a Kroki server, Mermaid via mermaid.js. Without the
  flag, diagram fences stay plain code blocks.
- `--safe` escapes raw HTML instead of emitting it. Off by default. See
  [Raw HTML and `--safe`](#raw-html-and---safe).
- `--static` renders self-contained HTML: interactive constructs are flattened
  and diagrams/math degrade to their source. Implies `--extensions`. (carve-go
  has no build-time image renderer over the WASI boundary, so static mode
  degrades diagrams to source rather than embedding an image.)
- `--profile NAME` restricts what a document may contain and how large it may
  be. Off by default. See [Profiles and their input caps](#profiles-and-their-input-caps).
- `--symbols FILE` and `--symbol NAME=VALUE` supply the symbol map that decides
  what `:name:` renders as. See [Symbols](#symbols).

`hugo-carve` takes flags and nothing else. An operand is refused with a message
rather than ignored, because Go's flag parsing stops at the first non-flag
argument: `hugo-carve content --safe` would otherwise read `content` as an
operand, never apply `--safe`, and exit 0 having passed raw HTML through.

## Raw HTML and `--safe`

Carve renders a `=html` block and a `` `...`{=html} `` span verbatim, by
design - that is what raw passthrough is for. `--safe` escapes them instead, so
they reach the page as visible text:

````
Before.

```=html
<div class="x"><script>alert(1)</script></div>
```

Inline: `<b>bold</b>`{=html} tail.
````

Without `--safe` (the default):

```html
<p>Before.</p>
<div class="x"><script>alert(1)</script></div>
<p>Inline: <b>bold</b> tail.</p>
```

With `--safe`:

```html
<p>Before.</p>
&lt;div class="x"&gt;&lt;script&gt;alert(1)&lt;/script&gt;&lt;/div&gt;
<p>Inline: &lt;b&gt;bold&lt;/b&gt; tail.</p>
```

**Default: off.** A site that adds nothing to its build command renders exactly
what it rendered before this flag existed. Turn it on for any tree whose pages
can come from outside the site's own authors - a docs site taking
contributions, or any build where a page can arrive in a pull request:

```bash
hugo-carve --content content --safe && hugo
```

### What `--safe` does and does not cover

It covers raw HTML passthrough, and that is the whole of it - because that is
the whole of what needs a switch. Carve's hardening is **always on** and is not
what this flag controls:

| | without `--safe` | with `--safe` |
|---|---|---|
| `=html` block, `{=html}` span | emitted verbatim | escaped to text |
| `javascript:` link destination | already blanked | blanked |
| event-handler attribute (`onclick=`) | already dropped | dropped |
| bidi override characters (Trojan Source) | already removed | removed |
| a symbol value from `--symbol` / `--symbols` | inserted raw | **inserted raw** |

That last row is the one to read twice. `--safe` is a statement about the
**document**; a symbol value is **site configuration**, so it is substituted
unescaped either way. `--safe` is not a license to build a symbol map out of
page content - see the security note under [Symbols](#symbols).

Two further limits worth stating plainly. `--safe` is a rendering option, not a
sandbox: it says nothing about what your Hugo templates, shortcodes or theme
put on the page around the converted body. And it applies to what `hugo-carve`
converts, so a `.html` file already sitting in `content/` is passed through by
Hugo untouched, whatever this flag says.

## Profiles and their input caps

`--profile NAME` hands the engine one of its own profile names, which restricts
what a document may contain. Off by default, and the empty default renders
exactly what this tool has always rendered.

```bash
hugo-carve --content content --profile article && hugo
```

What each name does, measured against the pinned engine:

| profile | links | images | raw HTML | input cap |
| --- | --- | --- | --- | --- |
| off (default) | kept | kept | emitted unless `--safe` | none |
| `full` | kept | kept | emitted unless `--safe` | none |
| `article` | kept | kept | escaped | none |
| `comment` | kept, with `rel="nofollow ugc"` | degraded to `[img: ALT]` | escaped | 100000 bytes |
| `minimal` | reduced to their text | degraded to `[img: ALT]` | escaped | 10000 bytes |

`full` is the engine's full behavior and renders exactly what no profile at all
renders, so it is a way to say the choice was deliberate rather than a change in
output. For a Hugo *page* the useful names are `article` and `full`, and neither
caps anything. `comment` and `minimal` are shaped for user-submitted snippets; they
work on a page tree, but the cap below applies.

The name is passed through as written. An unknown or wrongly-cased one comes
back as the engine's own message, and names are case-sensitive:

```
$ hugo-carve --content content --profile COMMENT
hugo-carve: convert "content/page.crv": render carve body: carve: engine exited with code 1: carve: unknown profile: COMMENT (expected full|article|comment|minimal)
$ echo $?
1
```

### An over-cap page stops the build

The cap counts BYTES of the page BODY - front matter is split off before the
engine sees anything and does not count toward it - and the count is bytes, not
characters, so 5001 two-byte characters is 10001 bytes and over the `minimal`
cap.

A body over the cap is an ERROR that names the cap and the actual size, and no
`.html` is written:

```
$ hugo-carve --content content --profile minimal
hugo-carve: convert "content/page.crv": profile "minimal" discarded the whole body: it is 10001 bytes and the "minimal" profile caps input at 10000 bytes (front matter is not counted). Split the page, or render it with the "article" or "full" profile
$ echo $?
1
```

That refusal is the whole point of the flag being safe to use. The engine
embedded in the pinned `carve-go` answers an over-cap document with an EMPTY
render, exit status 0 and an empty stderr
([carve-rs#1190](https://github.com/markup-carve/carve-rs/issues/1190)), so
forwarding the option without the guard would write a blank page that Hugo
publishes, with a green build and nothing in the log. A build that stops with a
message beats a site that publishes an empty page.

## Symbols

Carve parses `:name:` in its core - no extension needed - but what a name
renders as is a render option. With no map configured, a shortcode renders as
its own source text:

```
Ship it :rocket: :shrug:
```

```html
<p>Ship it :rocket: :shrug:</p>
```

`--symbols FILE` supplies the map, as a JSON object. Keep it in your site
repository next to `hugo.toml` (or under `data/`, which Hugo already treats as
site data), so the map is part of the site's configuration:

```json
{
  "rocket": "🚀",
  "smile": "😄"
}
```

```bash
hugo-carve --content content --symbols data/carve-symbols.json && hugo
```

```html
<p>Ship it 🚀 :shrug:</p>
```

An unmapped name stays literal, as `:shrug:` does above - it never becomes an
error or an empty string.

Both flags are repeatable, and the sources merge left to right: every
`--symbols` file in the order given, then every `--symbol` pair. So a generated
map can carry a handful of site-specific overrides without being edited:

```bash
hugo-carve --content content \
  --symbols data/emoji.json \
  --symbol rocket=🚀 \
  --symbol logo='<img src="/logo.svg" alt="">'
```

The word-boundary guard is unaffected by a populated map. In one document with
`rocket` mapped, only the fourth of these substitutes:

```
a:rocket:b and 3:rocket:4 and `A :rocket: x` and A :rocket: here
```

A name or value the engine cannot pass through intact - an empty name, a name
containing `=`, a NUL in either half - is refused by `carve-go` with its own
message, and `hugo-carve` reports it and converts nothing. There is no second
set of rules here to disagree with the engine's.

> **Security: symbol values are TRUSTED RAW output.**
> A mapped value is inserted into the page **unescaped** - that is deliberate
> across every Carve engine, and it is what lets a symbol expand to markup such
> as an `<img>` tag. The map is therefore trusted processor configuration, on
> the same footing as your templates. Build it only from **your own site
> configuration** - a file in your site repository, or a `--symbol` flag in your
> build command. **Never** build it from page content, front matter, or anything
> else a document author supplies: a value is a script-injection vector.

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
ignoreFiles = ['\.crv$']
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

## Styling

Carve renders constructs such as admonitions, code groups, spoilers, math, and
mentions to HTML that needs CSS to look right. A ready-to-use stylesheet,
vendored from the canonical Carve theme, ships with the example at
[`example/assets/css/carve.css`](example/assets/css/carve.css). Its rules are
scoped to a `.carve-content` wrapper, so apply it by wrapping the rendered page
content in an element with that class and linking the stylesheet, as the example
layout does:

```html
<!-- example/layouts/_default/baseof.html -->
{{ with resources.Get "css/carve.css" }}
<link rel="stylesheet" href="{{ .RelPermalink }}">
{{ end }}
...
<main class="carve-content">{{ block "main" . }}{{ end }}</main>
```

The stylesheet is self-contained: it defines its color, font, and background
variables (`--vp-c-*`) up front, with light-mode defaults you can override in
your own CSS. Copy the file into your own site's `assets/` (or a theme) to reuse
it outside the example.

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
