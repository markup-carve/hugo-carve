# Changelog

Notable changes to `hugo-carve`.

Rendering is done by the Carve engine, which this repository never builds: it
links `carve-go`, and `carve-go` embeds a prebuilt WebAssembly module compiled
from `carve-rs`. An engine change can therefore alter output with no diff here,
so engine pin moves get an entry of their own.

## Unreleased

### Added

- A symbol map, so `:name:` renders as something on a Hugo site instead of as
  its own source text. `convert.Options.Symbols`, and `--symbols FILE` /
  `--symbol NAME=VALUE` on the command line (both repeatable, merged left to
  right). Values are substituted raw, so the map is site configuration only -
  see the security note in the README. (#16)

### Changed

- The `carve-go` pin moves to `v0.1.2-0.20260821023146-73f58ce7cef1`, which is
  the first revision carrying `Options.Symbols`.

## v0.1.0 - 2026-08-18

First release.

### Added

- Hugo preprocessor for the Carve markup language. `.crv` content is converted
  to HTML for Hugo to consume, with a leading front matter block passed through
  unchanged so Hugo's own parser still reads it.
- `hugo-carve` command line entry point under `cmd/hugo-carve`.

### Security

- The engine pin carries the Carve 0.1.3 security release, in which a
  list-valued URL attribute was only probed on its FIRST entry, so
  `srcset="safe.png 1x, javascript:alert(1) 2x"` passed sanitization on the
  second one. Nothing was ever published from this repository, so this is the
  floor the first release ships on rather than a fix to an installed version.

### Changed

- The `carve-go` pin moves to `v0.1.1-0.20260818085012-4ca3e628a17b`, the
  release-freeze rebuild of the embedded engine. All 1213 comparable spec
  corpus documents render identically through it, and identically to what
  `carve-go` main produces.
