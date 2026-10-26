# hugo-carve

Hugo preprocessor for Carve pages, using carve-go v0.1.8. It converts `.crv` content before Hugo builds the site. Read the [Carve documentation](https://markup-carve.github.io/carve/) for the language syntax.

## Install

```sh
go install github.com/markup-carve/hugo-carve/cmd/hugo-carve@latest
```

## Usage

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
