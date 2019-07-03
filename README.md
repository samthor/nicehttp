Convenience library for HTTP handlers in Go.
These help [@samthor](https://twitter.com/samthor) write servers.
YMMV.

See [the godoc](https://godoc.org/github.com/samthor/nicehttp) for documentation.

## Convenience handler type

The core `Handler` type looks like this and implements `http.Handler`:

```go
type Handler func(ctx context.Context, r *http.Request) interface{}
```

As you write HTTP handlers, you can just return a variety of types to have your HTTP server do something sensible.
If you return an `error`, the server will serve a 500 and `log` the error.

See [the godoc](https://godoc.org/github.com/samthor/nicehttp#Handler) for more supported types.

Yes, this prevents static type checking for your HTTP handlers.
Yes, the convenience is worth it.

## App Engine

The `dev_appserver` script seems to be going away for runtimes of `go112` and beyond, which means your program will no longer read its "app.yaml" in development.
Static handlers will still work in production ¯\\_\(ツ\)_/¯

Run `nicehttp.Serve("app.yaml", nil)` to host your application with:

* static handler support in development, and
* automatic serving on the environment variable `PORT`
