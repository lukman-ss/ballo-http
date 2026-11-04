// Package ballo provides HTTP abstraction and middleware primitives
// for the Ballo framework ecosystem.
package ballo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const (
	paramsKey contextKey = iota
	requestIDKey
)

// Handler is the core function type processed by the middleware chain.
type Handler func(*Request) (*Response, error)

// Middleware wraps a Handler, enabling short-circuit on error or early return.
type Middleware func(Handler) Handler

// Chain applies middlewares left-to-right (outermost first).
func Chain(h Handler, mws ...Middleware) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// ─── Request ─────────────────────────────────────────────────────────────────

// Request wraps *http.Request with context and convenience helpers.
type Request struct {
	raw *http.Request
	ctx context.Context
}

// NewRequest constructs a Request from a standard *http.Request.
func NewRequest(r *http.Request) *Request {
	return &Request{raw: r, ctx: r.Context()}
}

// Raw returns the underlying *http.Request.
func (r *Request) Raw() *http.Request { return r.raw }

// Context returns the request's context.
func (r *Request) Context() context.Context { return r.ctx }

// WithContext returns a shallow copy of the Request with the given context.
func (r *Request) WithContext(ctx context.Context) *Request {
	return &Request{raw: r.raw.WithContext(ctx), ctx: ctx}
}

// Param returns a path parameter stored in context by the router layer.
// Returns empty string when key absent.
func (r *Request) Param(key string) string {
	m, _ := r.ctx.Value(paramsKey).(map[string]string)
	return m[key]
}

// WithParams returns a new Request with the given path-parameter map attached.
func WithParams(r *Request, params map[string]string) *Request {
	ctx := context.WithValue(r.ctx, paramsKey, params)
	return r.WithContext(ctx)
}

// Query returns the first value for the named query parameter.
func (r *Request) Query(key string) string {
	return r.raw.URL.Query().Get(key)
}

// QueryAll returns all values for the named query parameter.
func (r *Request) QueryAll(key string) []string {
	return r.raw.URL.Query()[key]
}

// Header returns the first value for the named request header.
func (r *Request) Header(key string) string {
	return r.raw.Header.Get(key)
}

// DecodeJSON decodes the JSON request body into v.
func (r *Request) DecodeJSON(v any) error {
	defer r.raw.Body.Close()
	return json.NewDecoder(r.raw.Body).Decode(v)
}

// Method returns the HTTP method of the request.
func (r *Request) Method() string { return r.raw.Method }

// Path returns the URL path of the request.
func (r *Request) Path() string { return r.raw.URL.Path }

// ─── Response ────────────────────────────────────────────────────────────────

// Response represents an HTTP response ready to be written.
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	committed  bool
}

// newResponse allocates a Response with initialised headers.
func newResponse(code int, body []byte) *Response {
	return &Response{
		StatusCode: code,
		Headers:    make(http.Header),
		Body:       body,
	}
}

// SetHeader sets a response header (must be called before Write).
func (resp *Response) SetHeader(key, value string) {
	resp.Headers.Set(key, value)
}

// Committed reports whether Write has already been called.
func (resp *Response) Committed() bool { return resp.committed }

// Write flushes the response to w exactly once; subsequent calls are no-ops.
func (resp *Response) Write(w http.ResponseWriter) {
	if resp.committed {
		return
	}
	resp.committed = true
	for k, vs := range resp.Headers {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if len(resp.Body) > 0 {
		_, _ = w.Write(resp.Body)
	}
}

// ─── Response helpers ─────────────────────────────────────────────────────────

// JSON encodes v as JSON and returns a Response with Content-Type application/json.
func JSON(code int, v any) (*Response, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("ballo: JSON marshal: %w", err)
	}
	r := newResponse(code, b)
	r.Headers.Set("Content-Type", "application/json; charset=utf-8")
	return r, nil
}

// Text returns a plain-text Response.
func Text(code int, s string) (*Response, error) {
	r := newResponse(code, []byte(s))
	r.Headers.Set("Content-Type", "text/plain; charset=utf-8")
	return r, nil
}

// HTML returns an HTML Response.
func HTML(code int, html string) (*Response, error) {
	r := newResponse(code, []byte(html))
	r.Headers.Set("Content-Type", "text/html; charset=utf-8")
	return r, nil
}

// NoContent returns a 204 No Content Response with no body.
func NoContent() (*Response, error) {
	return newResponse(http.StatusNoContent, nil), nil
}

// Error returns a JSON error response with the given status code and message.
func Error(code int, message string) (*Response, error) {
	return JSON(code, map[string]string{"error": message})
}

// ─── ServeHTTP adapter ────────────────────────────────────────────────────────

// Adapt converts a Handler into a standard http.HandlerFunc.
// On handler error it writes a 500 JSON body; on nil Response it writes 200.
func Adapt(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := NewRequest(r)
		resp, err := h(req)
		if err != nil {
			resp, _ = Error(http.StatusInternalServerError, "internal server error")
		}
		if resp == nil {
			resp = newResponse(http.StatusOK, nil)
		}
		resp.Write(w)
	}
}

// MethodNotAllowed returns a 405 Response with Allow header.
func MethodNotAllowed(allowed ...string) (*Response, error) {
	r := newResponse(http.StatusMethodNotAllowed, nil)
	if len(allowed) > 0 {
		r.Headers.Set("Allow", strings.Join(allowed, ", "))
	}
	return r, nil
}

// NotFound returns a 404 Response.
func NotFound() (*Response, error) {
	return Error(http.StatusNotFound, "not found")
}
