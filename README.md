# hugo-carve

Hugo preprocessor for Carve pages, using the Carve 0.2 engine from carve-go `main` (unreleased). It converts `.crv` content before Hugo builds the site. Read the [Carve documentation](https://markup-carve.github.io/carve/) for the language syntax.

## Install

```sh
go install github.com/markup-carve/hugo-carve/cmd/hugo-carve@latest
```

## Usage

Add this to your site's `hugo.toml`, above any `[table]`. Hugo refuses HTML content pages without it, and `ignoreFiles` keeps the raw `.crv` sources out of `public/`:

```toml
ignoreFiles = ['\.crv$']

[security]
allowContent = ['^text/html$']
```

Then convert and build:

```sh
hugo-carve --content content
hugo
```

## Documentation

- [Carve documentation](https://markup-carve.github.io/carve/)
- [Documentation index](docs/README.md)
- [Reference](docs/reference.md)
- [Development](docs/development.md)

## License

[MIT](LICENSE)
