package toolid

import (
	"strings"
	"testing"
)

func TestMapID_passthroughNineAlnum(t *testing.T) {
	id := "AbC123xyz"
	if got := MapID(id); got != id {
		t.Fatalf("got %q", got)
	}
}

func TestMapID_openaiStyleIsStable(t *testing.T) {
	a := MapID("call_PTLP8xhu3uwZk4l3nlnrrJha")
	b := MapID("call_PTLP8xhu3uwZk4l3nlnrrJha")
	if a != b {
		t.Fatalf("unstable %q vs %q", a, b)
	}
	if len(a) != 9 {
		t.Fatalf("len=%d", len(a))
	}
	for _, c := range a {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			t.Fatalf("invalid char %q in %q", c, a)
		}
	}
}

func TestRewriteBody_rewritesToolCallAndToolResult(t *testing.T) {
	in := []byte(`{"model":"mistral-small-latest","messages":[{"role":"assistant","tool_calls":[{"id":"call_abc","type":"function","function":{"name":"x","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_abc","content":"ok"}]}`)
	out1, err := RewriteBody(in)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := RewriteBody(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(out1) != string(out2) {
		t.Fatal("rewrite must be stable")
	}
	if strings.Contains(string(out1), "call_abc") {
		t.Fatalf("original id still present: %s", out1)
	}
	mapped := MapID("call_abc")
	if !strings.Contains(string(out1), mapped) {
		t.Fatalf("mapped id %q missing in %s", mapped, out1)
	}
}

func TestRewriteBody_leavesValidIDs(t *testing.T) {
	in := []byte(`{"messages":[{"role":"assistant","tool_calls":[{"id":"AbC123xyz","type":"function","function":{"name":"x","arguments":"{}"}}]}]}`)
	out, err := RewriteBody(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "AbC123xyz") {
		t.Fatalf("lost valid id: %s", out)
	}
}
