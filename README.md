# mqube-go-common
Place for common tooling for the Go language (Go go go go go)

## Installation
To add `mqube-go-common` to your Go project, run:

```sh
go get github.com/mikelear/leartech-bus-common
```

This will add the module to your go.mod file.

To add a specific package from the module, use:

```go
import (
    "github.com/mikelear/leartech-bus-common/pkg/cache"
)
```

To update to the latest version later, use:

```sh
go get -u github.com/mikelear/leartech-bus-common
```

## Using a Local Module for Development
When developing common code for use across multiple Go projects, you might want to test changes locally before pushing them to a remote repository.

This can be achieved by replacing the remote version of mqube-go-common with your local repository.
Add a `replace` directive to your `go.mod`:

```
replace github.com/mikelear/leartech-bus-common => ../mqube-go-common
```

- Keep the original reference to the remote module in your `require` section.
- The path can be relative or absolute and should point to your local repository.
- Run `go mod tidy` after adding the replace.
- Remove the `replace` line when you want to use the remote version again.

See the [Go Modules Reference](https://golang.org/ref/mod#go-mod-file-replace) for more details.
