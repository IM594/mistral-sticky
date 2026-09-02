package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IM594/mistral-sticky/internal/pool"
)

const testToken = "test-proxy-token"

func TestHealthz(t *testing.T) {
	h, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true {
		t.Fatalf("body=%v", body)
	}
}

func TestUnauthorized(t *testing.T) {
	h, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called")
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"mistral-small-latest","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestStickySameSessionSameKey(t *testing.T) {
	var auths []string
	h, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	body := `{"model":"mistral-small-latest","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"}]}`
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+testToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	if len(auths) != 3 || auths[0] != auths[1] || auths[1] != auths[2] {
		t.Fatalf("auths=%v", auths)
	}
	if !strings.HasPrefix(auths[0], "Bearer key-") {
		t.Fatalf("unexpected auth %q", auths[0])
	}
}

func TestUnauthorizedUpstreamDisablesKey(t *testing.T) {
	calls := 0
	h, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	body := `{"model":"mistral-small-latest","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("first status=%d", rec.Code)
	}
	req2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+testToken)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("second status=%d", rec2.Code)
	}
}

func TestRateLimitDoesNotRotateKey(t *testing.T) {
	var auths []string
	h, p := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
		if len(auths) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	body := `{"model":"mistral-small-latest","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("first status=%d", rec.Code)
	}
	if p.DisabledCount() != 0 {
		t.Fatalf("429 must not disable keys, disabled=%d", p.DisabledCount())
	}
	req2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+testToken)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("second status=%d", rec2.Code)
	}
	if len(auths) != 2 || auths[0] != auths[1] {
		t.Fatalf("expected same upstream key after 429, auths=%v", auths)
	}
}

func TestDropsUnsupportedReasoningEffort(t *testing.T) {
	var got []byte
	h, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	body := `{"model":"mistral-medium-latest","reasoning_effort":"medium","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	if strings.Contains(string(got), "reasoning_effort") {
		t.Fatalf("reasoning_effort leaked upstream: %s", got)
	}
}

func TestInjectsStablePromptCacheKey(t *testing.T) {
	var keys []string
	h, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var parsed struct {
			PromptCacheKey string `json:"prompt_cache_key"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Errorf("body=%s err=%v", raw, err)
		}
		keys = append(keys, parsed.PromptCacheKey)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	turn1 := `{"model":"mistral-small-latest","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"}]}`
	turn2 := `{"model":"mistral-small-latest","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"},{"role":"assistant","content":"ok"},{"role":"user","content":"more"}]}`
	for _, body := range []string{turn1, turn2} {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+testToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("status=%d", rec.Code)
		}
	}
	if len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("prompt_cache_key not stable: %v", keys)
	}
}

func TestUpstream5xxPassedThrough(t *testing.T) {
	h, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"mistral-small-latest","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNoAvailableKey(t *testing.T) {
	h, p := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called")
	})
	for i := 0; i < p.Size(); i++ {
		p.Disable(i, time.Hour, "401")
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"mistral-small-latest","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestUpstreamDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	srv.Close()
	dir := t.TempDir()
	keysPath := filepath.Join(dir, "keys.txt")
	if err := os.WriteFile(keysPath, []byte("key-a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := pool.LoadKeys(keysPath)
	if err != nil {
		t.Fatal(err)
	}
	h := New(Config{ProxyToken: testToken, Upstream: u, Pool: p})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"mistral-small-latest","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestForwardsBodyWithoutLeakingProxyToken(t *testing.T) {
	h, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer "+testToken {
			t.Error("proxy token leaked upstream")
		}
		got, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(got), `"hello"`) {
			t.Errorf("body not forwarded: %s", got)
		}
		w.WriteHeader(200)
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"mistral-small-latest","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
}

func newTestHandler(t *testing.T, upstream http.HandlerFunc) (http.Handler, *pool.Pool) {
	t.Helper()
	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	keysPath := filepath.Join(dir, "keys.txt")
	if err := os.WriteFile(keysPath, []byte("key-a\nkey-b\nkey-c\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := pool.LoadKeys(keysPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SetCooldownFile(filepath.Join(dir, "cooldown.json")); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return New(Config{ProxyToken: testToken, Upstream: u, Pool: p}), p
}
