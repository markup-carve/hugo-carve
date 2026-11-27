# Development

## Setup and maintenance

To develop against a local `carve-go` checkout, temporarily add a `replace`
directive using the checkout's actual path, for example:

```
replace github.com/markup-carve/carve-go => ../carve-go
```

Do not commit the local `replace`. `go.mod` requires a carve-go 0.2 release,
and the code imports its root package `github.com/markup-carve/carve-go`.
Move to the newest 0.2 release with:

```
go get github.com/markup-carve/carve-go@v0.2
```

Run the tests:

```bash
go build ./...
go vet ./...
go test ./...
```
