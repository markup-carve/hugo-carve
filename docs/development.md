# Development

## Setup and maintenance

To develop against a local `carve-go` checkout, temporarily add a `replace`
directive using the checkout's actual path, for example:

```
replace github.com/markup-carve/carve-go => ../carve-go
```

Do not commit the local `replace`. On `next2`, `go.mod` pins an unreleased
carve-go `next2` commit by pseudo-version, and the code imports its Carve 0.2
package `github.com/markup-carve/carve-go/v2`. Move the pin with:

```
go get github.com/markup-carve/carve-go@<next2 commit>
```

Run the tests:

```bash
go build ./...
go vet ./...
go test ./...
```
