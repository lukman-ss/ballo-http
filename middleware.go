package ballo

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// ─── Recovery ────────────────────────────────────────────────────────────────

// Recovery catches panics in downstream handlers, logs the error safely
// (no stack trace leaked to the client), and returns HTTP 500.
func Recovery() Middleware {
	return func(next Handler) Handler {
		return func(r *Request) (resp *Response, err error) {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("ballo: panic recovered", "error", fmt.Sprintf("%v", rec))
					resp, err = Error(http.StatusInternalServerError, "internal server error")
				}
			}()
			return next(r)
		}
	}
}

// ─── RequestID ───────────────────────────────────────────────────────────────

// RequestID reads X-Request-ID from the incoming request or generates a new
// UUID v4, stores it in the request context, and echoes it in the response.
func RequestID() Middleware {
	return func(next Handler) Handler {
		return func(r *Request) (*Response, error) {
			id := r.Header("X-Request-ID")
			if id == "" {
				id = newRequestID()
			}
			ctx := context.WithValue(r.ctx, requestIDKey, id)
			r = r.WithContext(ctx)
			resp, err := next(r)
			if resp != nil {
				resp.Headers.Set("X-Request-ID", id)
			}
			return resp, err
		}
	}
}

// RequestIDFromContext retrieves the request ID stored by the RequestID middleware.
// Returns empty string when absent.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// newRequestID generates a random UUID v4 string using crypto/rand.
func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ─── Logger ──────────────────────────────────────────────────────────────────

// Logger logs each request's method, path, status code, and latency using
// the structured slog default logger.
func Logger() Middleware {
	return func(next Handler) Handler {
		return func(r *Request) (*Response, error) {
			start := time.Now()
			resp, err := next(r)
			status := http.StatusOK
			if resp != nil {
				status = resp.StatusCode
			}
			if err != nil {
				status = http.StatusInternalServerError
			}
			slog.Info("ballo: request",
				"method", r.Method(),
				"path", r.Path(),
				"status", status,
				"latency", time.Since(start).String(),
				"request_id", RequestIDFromContext(r.Context()),
			)
			return resp, err
		}
	}
}
