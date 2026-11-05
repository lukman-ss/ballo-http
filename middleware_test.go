package ballo

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── Recovery ────────────────────────────────────────────────────────────────

func TestRecovery_NoPanic(t *testing.T) {
	h := Chain(func(r *Request) (*Response, error) {
		return Text(http.StatusOK, "ok")
	}, Recovery())
	resp, err := h(newTestRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestRecovery_Panic(t *testing.T) {
	h := Chain(func(r *Request) (*Response, error) {
		panic("something broke")
	}, Recovery())
	resp, err := h(newTestRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal("err should be nil after recovery, got:", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
	// stack trace must not leak to response body
	if strings.Contains(string(resp.Body), "goroutine") {
		t.Fatal("stack trace leaked in response body")
	}
}

// ─── RequestID ───────────────────────────────────────────────────────────────

func TestRequestID_Generated(t *testing.T) {
	var capturedID string
	h := Chain(func(r *Request) (*Response, error) {
		capturedID = RequestIDFromContext(r.Context())
		return NoContent()
	}, RequestID())
	resp, err := h(newTestRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if capturedID == "" {
		t.Fatal("request ID not set in context")
	}
	if resp.Headers.Get("X-Request-ID") != capturedID {
		t.Fatal("X-Request-ID header mismatch")
	}
}

func TestRequestID_Passthrough(t *testing.T) {
	const existing = "my-custom-id"
	raw, _ := http.NewRequest(http.MethodGet, "/", nil)
	raw.Header.Set("X-Request-ID", existing)
	req := NewRequest(raw)

	var capturedID string
	h := Chain(func(r *Request) (*Response, error) {
		capturedID = RequestIDFromContext(r.Context())
		return NoContent()
	}, RequestID())
	h(req) //nolint:errcheck
	if capturedID != existing {
		t.Fatalf("want %s, got %s", existing, capturedID)
	}
}

func TestRequestID_Uniqueness(t *testing.T) {
	seen := map[string]bool{}
	h := Chain(func(r *Request) (*Response, error) {
		return NoContent()
	}, RequestID())
	for i := 0; i < 100; i++ {
		resp, _ := h(newTestRequest(http.MethodGet, "/", nil))
		id := resp.Headers.Get("X-Request-ID")
		if seen[id] {
			t.Fatalf("duplicate request ID: %s", id)
		}
		seen[id] = true
	}
}

// ─── Logger ──────────────────────────────────────────────────────────────────

func TestLogger_DoesNotMutateResponse(t *testing.T) {
	h := Chain(func(r *Request) (*Response, error) {
		return JSON(http.StatusCreated, map[string]string{"k": "v"})
	}, Logger())
	resp, err := h(newTestRequest(http.MethodPost, "/items", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
}

func TestLogger_WithError(t *testing.T) {
	h := Chain(func(r *Request) (*Response, error) {
		return nil, errTest("logger test error")
	}, Logger())
	_, err := h(newTestRequest(http.MethodGet, "/", nil))
	if err == nil {
		t.Fatal("expected error propagated")
	}
}

// ─── newRequestID ─────────────────────────────────────────────────────────────

func TestNewRequestID_Format(t *testing.T) {
	id, err := newRequestID()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// UUID v4: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx (36 chars)
	if len(id) != 36 {
		t.Fatalf("want length 36, got %d: %s", len(id), id)
	}
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("want 5 parts, got %d", len(parts))
	}
}

func TestRequestID_CryptoRandFailure(t *testing.T) {
	orig := readRand
	readRand = func([]byte) (int, error) { return 0, fmt.Errorf("rand fail") }
	defer func() { readRand = orig }()

	h := Chain(func(r *Request) (*Response, error) {
		return NoContent()
	}, RequestID())
	resp, err := h(newTestRequest(http.MethodGet, "/", nil))
	if err == nil {
		t.Fatal("expected error from crypto/rand failure")
	}
	if resp != nil {
		t.Fatal("expected nil response on request id failure")
	}
	if !strings.Contains(err.Error(), "rand fail") {
		t.Fatalf("expected rand failure propagated, got: %v", err)
	}
}

func TestRequestID_CryptoRandFailure_Now(t *testing.T) {
	orig := readRand
	readRand = func([]byte) (int, error) { return 15, nil }
	defer func() { readRand = orig }()

	h := Chain(func(r *Request) (*Response, error) {
		return NoContent()
	}, RequestID())
	resp, err := h(newTestRequest(http.MethodGet, "/", nil))
	if err == nil {
		t.Fatal("expected error from short crypto/rand read")
	}
	if resp != nil {
		t.Fatal("expected nil response on request id failure")
	}
}

// TestRequestID_RandReadError_confirmsInjectability confirms readRand injection works for tests.
func TestRequestID_RandReadError_confirmsInjectability(t *testing.T) {
	orig := readRand
	readRand = func([]byte) (int, error) { return 0, fmt.Errorf("injected rand error") }
	defer func() { readRand = orig }()

	h := Chain(func(r *Request) (*Response, error) {
		return NoContent()
	}, RequestID())
	resp, err := h(newTestRequest(http.MethodGet, "/", nil))
	if err == nil {
		t.Fatal("expected error from injected rand failure")
	}
	if !strings.Contains(err.Error(), "injected rand error") {
		t.Fatalf("expected injected error propagated, got: %v", err)
	}
	if resp != nil {
		t.Fatal("expected nil response on rand error")
	}
}

// ─── Full stack integration ───────────────────────────────────────────────────

func TestFullStack(t *testing.T) {
	h := Chain(
		func(r *Request) (*Response, error) {
			id := RequestIDFromContext(r.Context())
			return JSON(http.StatusOK, map[string]string{"request_id": id})
		},
		Recovery(),
		RequestID(),
		Logger(),
	)

	srv := httptest.NewServer(Adapt(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID missing from response")
	}
}
