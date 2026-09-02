package session

import (
	"net/http"
	"strings"
	"testing"
)

func TestKey_prefersMetadataSessionID(t *testing.T) {
	body := []byte(`{"model":"mistral-small-latest","metadata":{"session_id":"sess-1","user_id":"u-1"},"messages":[{"role":"user","content":"hi"}]}`)
	got := Key(http.Header{}, body)
	if got != "sess-1" {
		t.Fatalf("got %q", got)
	}
}

func TestKey_metadataUserID(t *testing.T) {
	body := []byte(`{"model":"mistral-small-latest","metadata":{"user_id":"u-9"},"messages":[{"role":"user","content":"hi"}]}`)
	got := Key(http.Header{}, body)
	if got != "u-9" {
		t.Fatalf("got %q", got)
	}
}

func TestKey_conversationID(t *testing.T) {
	body := []byte(`{"model":"mistral-small-latest","conversation_id":"c-2","messages":[{"role":"user","content":"hi"}]}`)
	got := Key(http.Header{}, body)
	if got != "c-2" {
		t.Fatalf("got %q", got)
	}
}

func TestKey_promptCacheKey(t *testing.T) {
	body := []byte(`{"model":"mistral-small-latest","prompt_cache_key":"pck-3","messages":[{"role":"user","content":"hi"}]}`)
	got := Key(http.Header{}, body)
	if got != "pck-3" {
		t.Fatalf("got %q", got)
	}
}

func TestKey_headerSessionID(t *testing.T) {
	h := http.Header{}
	h.Set("X-Session-ID", "hdr-4")
	body := []byte(`{"model":"mistral-small-latest","messages":[{"role":"user","content":"hi"}]}`)
	got := Key(h, body)
	if got != "hdr-4" {
		t.Fatalf("got %q", got)
	}
}

func TestKey_headerSessionIdAlias(t *testing.T) {
	h := http.Header{}
	h.Set("Session-Id", "hdr-5")
	body := []byte(`{"model":"mistral-small-latest","messages":[{"role":"user","content":"hi"}]}`)
	got := Key(h, body)
	if got != "hdr-5" {
		t.Fatalf("got %q", got)
	}
}

func TestKey_messageHashStableAcrossTurns(t *testing.T) {
	turn1 := []byte(`{"model":"mistral-small-latest","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"}]}`)
	turn2 := []byte(`{"model":"mistral-small-latest","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"},{"role":"assistant","content":"ok"},{"role":"user","content":"more"}]}`)
	k1 := Key(nil, turn1)
	k2 := Key(nil, turn2)
	if k1 == "" || k1 != k2 {
		t.Fatalf("expected stable hash, got %q vs %q", k1, k2)
	}
}

func TestKey_arrayContent(t *testing.T) {
	body := []byte(`{"model":"mistral-small-latest","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	if Key(nil, body) == "" {
		t.Fatal("expected hash from array content")
	}
}

func TestFingerprint_length(t *testing.T) {
	fp := Fingerprint("sess-1")
	if len(fp) != 12 {
		t.Fatalf("len=%d", len(fp))
	}
}

func TestInjectPromptCacheKey_addsStableKey(t *testing.T) {
	in := []byte(`{"model":"mistral-small-latest","messages":[{"role":"user","content":"hello"}]}`)
	out := InjectPromptCacheKey(in, "sess-42")
	if string(out) == string(in) {
		t.Fatal("expected body to gain prompt_cache_key")
	}
	got := Key(nil, out)
	if got != "sess-42" {
		t.Fatalf("session key=%q", got)
	}
}

func TestInjectPromptCacheKey_idempotent(t *testing.T) {
	in := []byte(`{"model":"mistral-small-latest","prompt_cache_key":"sess-42","messages":[{"role":"user","content":"hello"}]}`)
	out := InjectPromptCacheKey(in, "sess-42")
	if string(out) != string(in) {
		t.Fatalf("expected original bytes, got %s", out)
	}
}

func TestInjectPromptCacheKey_preservesMessageBytes(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":"a < b & c"}],"model":"mistral-small-latest"}`)
	out := InjectPromptCacheKey(in, "sess-42")
	if !strings.Contains(string(out), `"content":"a < b & c"`) {
		t.Fatalf("message bytes mutated: %s", out)
	}
}
