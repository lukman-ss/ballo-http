package ballo

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── Request tests ────────────────────────────────────────────────────────────

func TestRequest_Param(t *testing.T) {
	r := newTestRequest(http.MethodGet, "/", nil)
	r = WithParams(r, map[string]string{"id": "42"})
	if got := r.Param("id"); got != "42" {
		t.Fatalf("want 42, got %s", got)
	}
	if got := r.Param("missing"); got != "" {
		t.Fatalf("want empty, got %s", got)
	}
}

func TestRequest_Query(t *testing.T) {
	r := newTestRequest(http.MethodGet, "/?foo=bar&foo=baz", nil)
	if got := r.Query("foo"); got != "bar" {
		t.Fatalf("want bar, got %s", got)
	}
	all := r.QueryAll("foo")
	if len(all) != 2 {
		t.Fatalf("want 2, got %d", len(all))
	}
	if got := r.Query("absent"); got != "" {
		t.Fatalf("want empty, got %s", got)
	}
}

func TestRequest_DecodeJSON_BodyTooLarge(t *testing.T) {
	body := strings.Repeat("x", int(MaxJSONBodySize)+1)
	r := newTestRequest(http.MethodPost, "/", strings.NewReader(body))
	var v any
	if err := r.DecodeJSON(&v); err == nil {
		t.Fatal("expected error for body too large")
	}
}

func TestRequest_DecodeJSON_BadHeader(t *testing.T) {
	raw, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader("ok"))
	raw.Header.Set("Content-Type", "text/plain")
	r := NewRequest(raw)
	var v any
	if err := r.DecodeJSON(&v); err == nil {
		t.Fatal("expected error for non-JSON content")
	}
}

func TestRequest_DecodeJSON_UnsupportedContentType(t *testing.T) {
	raw, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{"x":1}`))
	raw.Header.Set("Content-Type", "application/xml")
	r := NewRequest(raw)
	var v any
	err := r.DecodeJSON(&v)
	if err == nil {
		t.Fatal("expected error for non-JSON content type")
	}
	if !strings.Contains(err.Error(), "unsupported content type") {
		t.Fatalf("want unsupported content type, got: %v", err)
	}
}

func TestRequest_HeaderAll(t *testing.T) {
	raw, _ := http.NewRequest(http.MethodGet, "/", nil)
	raw.Header.Add("X-Multi", "first")
	raw.Header.Add("X-Multi", "second")
	r := NewRequest(raw)
	if got := r.Header("X-Multi"); got != "first" {
		t.Fatalf("want first, got %s", got)
	}
}

func TestRequest_DecodeJSON(t *testing.T) {
	body := `{"name":"ballo"}`
	r := newTestRequest(http.MethodPost, "/", strings.NewReader(body))
	var v map[string]string
	if err := r.DecodeJSON(&v); err != nil {
		t.Fatal(err)
	}
	if v["name"] != "ballo" {
		t.Fatalf("want ballo, got %s", v["name"])
	}
}

func TestRequest_DecodeJSON_InvalidBody(t *testing.T) {
	r := newTestRequest(http.MethodPost, "/", strings.NewReader("not-json"))
	var v any
	if err := r.DecodeJSON(&v); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRequest_DecodeJSON_EmptyBody(t *testing.T) {
	r := newTestRequest(http.MethodPost, "/", strings.NewReader(""))
	var v any
	if err := r.DecodeJSON(&v); err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestRequest_DecodeJSON_CustomLimit(t *testing.T) {
	body := strings.Repeat("a", 11)
	r := newTestRequest(http.MethodPost, "/", strings.NewReader(body))
	var v any
	if err := r.DecodeJSON(&v, 10); err == nil {
		t.Fatal("expected error for custom limit exceed")
	}
}

func TestRequest_DecodeJSON_TrailingData(t *testing.T) {
	body := `{"x":1}extra`
	r := newTestRequest(http.MethodPost, "/", strings.NewReader(body))
	var v map[string]int
	if err := r.DecodeJSON(&v); err == nil {
		t.Fatal("expected error for trailing data")
	}
}

func TestRequest_DecodeJSON_EmptyBodyAfterJSON(t *testing.T) {
	body := `{}` + strings.Repeat(" ", 10)
	r := newTestRequest(http.MethodPost, "/", strings.NewReader(body))
	var v map[string]any
	if err := r.DecodeJSON(&v); err != nil {
		t.Fatalf("expected success for valid empty object with whitespace, got: %v", err)
	}
}

func TestRequest_WithContext(t *testing.T) {
	r := newTestRequest(http.MethodGet, "/", nil)
	type key struct{}
	ctx := context.WithValue(r.Context(), key{}, "val")
	r2 := r.WithContext(ctx)
	if r2.Context().Value(key{}) != "val" {
		t.Fatal("context not propagated")
	}
}

func TestRequest_MethodAndPath(t *testing.T) {
	r := newTestRequest(http.MethodDelete, "/foo/bar", nil)
	if r.Method() != http.MethodDelete {
		t.Fatalf("want DELETE, got %s", r.Method())
	}
	if r.Path() != "/foo/bar" {
		t.Fatalf("want /foo/bar, got %s", r.Path())
	}
}

// ─── Response tests ───────────────────────────────────────────────────────────

func TestResponse_Write_Idempotent(t *testing.T) {
	resp := newResponse(http.StatusOK, []byte("hello"))
	w1 := httptest.NewRecorder()
	resp.Write(w1)
	w2 := httptest.NewRecorder()
	resp.Write(w2) // no-op
	if !resp.Committed() {
		t.Fatal("should be committed")
	}
	if w2.Code != 200 {
		// w2 never received WriteHeader; default recorder code is 200 but body empty
		t.Fatalf("unexpected code %d", w2.Code)
	}
	if w2.Body.Len() != 0 {
		t.Fatal("second Write should not write body")
	}
}

func TestResponse_SetHeader(t *testing.T) {
	resp := newResponse(http.StatusOK, nil)
	resp.SetHeader("X-Foo", "bar")
	w := httptest.NewRecorder()
	resp.Write(w)
	if got := w.Header().Get("X-Foo"); got != "bar" {
		t.Fatalf("want bar, got %s", got)
	}
}

// ─── Helper tests ─────────────────────────────────────────────────────────────

func TestJSON_Valid(t *testing.T) {
	resp, err := JSON(http.StatusOK, map[string]int{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Headers.Get("Content-Type"), "application/json") {
		t.Fatal("missing JSON content-type")
	}
	var m map[string]int
	if err := json.Unmarshal(resp.Body, &m); err != nil {
		t.Fatal(err)
	}
	if m["x"] != 1 {
		t.Fatalf("want 1, got %d", m["x"])
	}
}

func TestJSON_MarshalError(t *testing.T) {
	// channels are not JSON-serialisable
	_, err := JSON(200, make(chan int))
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestText(t *testing.T) {
	resp, err := Text(http.StatusOK, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Body) != "hello" {
		t.Fatalf("want hello, got %s", resp.Body)
	}
	if !strings.Contains(resp.Headers.Get("Content-Type"), "text/plain") {
		t.Fatal("missing text/plain content-type")
	}
}

func TestHTML(t *testing.T) {
	resp, err := HTML(http.StatusOK, "<h1>hi</h1>")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Headers.Get("Content-Type"), "text/html") {
		t.Fatal("missing text/html content-type")
	}
}

func TestNoContent(t *testing.T) {
	resp, err := NoContent()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}
	if len(resp.Body) != 0 {
		t.Fatal("body should be empty")
	}
}

func TestError(t *testing.T) {
	resp, err := Error(http.StatusBadRequest, "bad")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

// ─── Chain / Adapt tests ──────────────────────────────────────────────────────

func TestChain_Order(t *testing.T) {
	var order []string
	mk := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(r *Request) (*Response, error) {
				order = append(order, name+":before")
				resp, err := next(r)
				order = append(order, name+":after")
				return resp, err
			}
		}
	}
	h := Chain(func(r *Request) (*Response, error) {
		return NoContent()
	}, mk("A"), mk("B"))
	r := newTestRequest(http.MethodGet, "/", nil)
	h(r) //nolint:errcheck
	want := []string{"A:before", "B:before", "A:after", "B:after"}
	// B:after comes before A:after in unwinding
	want = []string{"A:before", "B:before", "B:after", "A:after"}
	if len(order) != 4 {
		t.Fatalf("want 4 events, got %v", order)
	}
	for i, w := range want {
		if order[i] != w {
			t.Fatalf("index %d: want %s, got %s", i, w, order[i])
		}
	}
}

func TestAdapt_NilResponse(t *testing.T) {
	hf := Adapt(func(r *Request) (*Response, error) { return nil, nil })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	hf(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatal("body should be empty for 204")
	}
}

func TestAdapt_HandlerError(t *testing.T) {
	hf := Adapt(func(r *Request) (*Response, error) {
		return nil, errTest("boom")
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	hf(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestJSON_Response(t *testing.T) {
	resp, err := JSON(http.StatusOK, map[string]string{"hello": "world"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Headers.Get("Content-Type"), "application/json") {
		t.Fatal("missing JSON content-type")
	}
	if string(resp.Body) != `{"hello":"world"}` {
		t.Fatalf("unexpected body: %s", resp.Body)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTestRequest(method, target string, body interface{ Read([]byte) (int, error) }) *Request {
	var b *bytes.Buffer
	if body != nil {
		b = new(bytes.Buffer)
		tmp := make([]byte, 4096)
		for {
			n, err := body.Read(tmp)
			b.Write(tmp[:n])
			if err != nil {
				break
			}
		}
	}
	var raw *http.Request
	if b != nil {
		raw, _ = http.NewRequest(method, target, b)
	} else {
		raw, _ = http.NewRequest(method, target, nil)
	}
	return NewRequest(raw)
}

type errTest string

func (e errTest) Error() string { return string(e) }
