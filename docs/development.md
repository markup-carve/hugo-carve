# Development

## Setup and maintenance

To develop against a local `carve-go` checkout, temporarily add a `replace`
directive using the checkout's actual path, for example:

```
replace github.com/markup-carve/carve-go => ../carve-go
```

Do not commit the local `replace`. Published code uses the released version
pinned by `go.mod`:

```
require github.com/markup-carve/carve-go v0.1.2
```

Run the tests:

```bash
go build ./...
go vet ./...
go test ./...
```
