# Changelog

Notable changes to `hugo-carve`.

Rendering is done by the Carve engine, which this repository never builds: it
links `carve-go`, and `carve-go` embeds a prebuilt WebAssembly module compiled
from `carve-rs`. An engine change can therefore alter output with no diff here,
so engine pin moves get an entry of their own.

## Unreleased

## v0.1.0 - 2026-09-09

First release.

### Added

- Hugo preprocessor for the Carve markup language. `.crv` content is converted
  to HTML for Hugo to consume, with a leading front matter block passed through
  unchanged so Hugo's own parser still reads it.
- `hugo-carve` command line entry point under `cmd/hugo-carve`. It takes flags
  only and refuses an unexpected operand: because Go's flag parsing stops at the
  first non-flag argument, `hugo-carve content --safe` would otherwise drop
  `--safe` silently and exit 0, so an operand is now a hard error. (#18)
- A symbol map, so `:name:` renders as something on a Hugo site instead of as
  its own source text. `convert.Options.Symbols`, and `--symbols FILE` /
  `--symbol NAME=VALUE` on the command line (both repeatable, merged left to
  right). Values are substituted raw, so the map is site configuration only -
  see the security note in the README. (#16)
- `--safe` / `convert.Options.Safe`, which escapes raw HTML (`=html` blocks and
  `{=html}` spans) instead of emitting it, so a site can render pages it did
  not author. Off by default. (#18)
- `--profile NAME` / `convert.Options.Profile`, forwarding the engine's
  `full|article|comment|minimal` profiles. Off by default. A body over the
  `comment` (100000 bytes) or `minimal` (10000 bytes) input cap is a hard error
  naming the cap and the actual size, instead of the engine's silent empty
  render that would publish a blank page with a green build
  (markup-carve/carve-rs#1190). (#20)

### Security

- The engine carries the Carve 0.1.3 security release, in which a list-valued
  URL attribute was only probed on its FIRST entry, so
  `srcset="safe.png 1x, javascript:alert(1) 2x"` passed sanitization on the
  second one. Nothing was ever published from this repository, so this is the
  floor the first release ships on rather than a fix to an installed version.

### Engine

- Rendering is provided by the released `carve-go` v0.1.2, which embeds the
  prebuilt Carve WebAssembly engine. (#22)
