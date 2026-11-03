# ballo-http

HTTP abstraction and middleware toolkit for the Ballo framework ecosystem.

## Installation

```bash
go get github.com/lukman-ss/ballo-http
```

Requires Go 1.23+. Zero external dependencies.

## Quick Start

```go
package main

import (
    "net/http"

    ballo "github.com/lukman-ss/ballo-http"
)

func main() {
    h := ballo.Chain(
        func(r *ballo.Request) (*ballo.Response, error) {
            return ballo.JSON(http.StatusOK, map[string]string{
                "hello": "world",
                "id":    ballo.RequestIDFromContext(r.Context()),
            })
        },
        ballo.Recovery(),
        ballo.RequestID(),
        ballo.Logger(),
    )

    http.ListenAndServe(":8080", ballo.Adapt(h))
}
```

## API Reference

### Core Types

```go
type Handler    func(*Request) (*Response, error)
type Middleware func(Handler) Handler
```

### `Chain(h Handler, mws ...Middleware) Handler`

Applies middlewares left-to-right (outermost first). The first middleware in the list is the outermost wrapper.

```go
h := ballo.Chain(myHandler, ballo.Recovery(), ballo.RequestID(), ballo.Logger())
```

### `Adapt(h Handler) http.HandlerFunc`

Bridges a `Handler` into the standard `net/http` ecosystem. On handler error → 500 JSON. On nil response → 200 empty body.

---

### Request

| Method | Description |
|--------|-------------|
| `Param(key string) string` | Path parameter set by the router via `WithParams`. |
| `Query(key string) string` | First query-string value for key. |
| `QueryAll(key string) []string` | All query-string values for key. |
| `Header(key string) string` | First request header value for key. |
| `DecodeJSON(v any) error` | Decodes JSON body into v and closes the body. |
| `Method() string` | HTTP method. |
| `Path() string` | URL path. |
| `Context() context.Context` | Active context. |
| `WithContext(ctx) *Request` | Returns shallow copy with new context. |
| `Raw() *http.Request` | Underlying `*http.Request`. |

#### Path Parameters

Set by a router layer (or tests) via the package-level helper:

```go
r = ballo.WithParams(r, map[string]string{"id": "42"})
id := r.Param("id") // "42"
```

---

### Response

| Method | Description |
|--------|-------------|
| `SetHeader(key, value string)` | Sets a response header (call before `Write`). |
| `Write(w http.ResponseWriter)` | Flushes headers and body exactly once (idempotent). |
| `Committed() bool` | True after the first `Write`. |

---

### Response Helpers

All helpers return `(*Response, error)`.

| Function | Status | Content-Type |
|----------|--------|--------------|
| `JSON(code, v)` | `code` | `application/json; charset=utf-8` |
| `Text(code, s)` | `code` | `text/plain; charset=utf-8` |
| `HTML(code, html)` | `code` | `text/html; charset=utf-8` |
| `NoContent()` | 204 | — |
| `Error(code, message)` | `code` | `application/json; charset=utf-8` |

---

### Middlewares

#### `Recovery() Middleware`

Catches any downstream `panic`, logs it with `slog.Error`, and returns HTTP 500 without leaking stack traces to the client.

#### `RequestID() Middleware`

- Reads `X-Request-ID` from the incoming request; generates a UUID v4 if absent.
- Stores the ID in the request context — retrieve with `RequestIDFromContext(ctx)`.
- Echoes the ID in the `X-Request-ID` response header.

```go
id := ballo.RequestIDFromContext(r.Context())
```

#### `Logger() Middleware`

Logs each request with `slog.Info` (structured):

```
INFO ballo: request method=GET path=/ping status=200 latency=1.2ms request_id=…
```

Fields: `method`, `path`, `status`, `latency`, `request_id`.

---

## Testing

```bash
go test -v -race ./...
go vet ./...
```

## License

MIT
