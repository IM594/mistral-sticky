package compat

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitize_dropsUnsupportedReasoningEffort(t *testing.T) {
	in := []byte(`{"model":"mistral-medium-latest","reasoning_effort":"medium","messages":[{"role":"user","content":"hi"}]}`)
	out := Sanitize(in)
	if strings.Contains(string(out), "reasoning_effort") {
		t.Fatalf("medium should be dropped: %s", out)
	}
	if !strings.Contains(string(out), `"content":"hi"`) {
		t.Fatalf("messages mutated: %s", out)
	}
}

func TestSanitize_keepsHighAndNone(t *testing.T) {
	for _, v := range []string{"high", "none"} {
		in, _ := json.Marshal(map[string]any{
			"model":            "mistral-small-latest",
			"reasoning_effort": v,
			"messages":         []map[string]string{{"role": "user", "content": "hi"}},
		})
		out := Sanitize(in)
		var parsed struct {
			ReasoningEffort string `json:"reasoning_effort"`
		}
		if err := json.Unmarshal(out, &parsed); err != nil {
			t.Fatal(err)
		}
		if parsed.ReasoningEffort != v {
			t.Fatalf("kept %q, got %q in %s", v, parsed.ReasoningEffort, out)
		}
	}
}

func TestSanitize_mapsMaxCompletionTokens(t *testing.T) {
	in := []byte(`{"model":"mistral-small-latest","max_completion_tokens":32,"messages":[{"role":"user","content":"hi"}]}`)
	out := Sanitize(in)
	var parsed struct {
		MaxTokens            *int `json:"max_tokens"`
		MaxCompletionTokens  *int `json:"max_completion_tokens"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.MaxTokens == nil || *parsed.MaxTokens != 32 {
		t.Fatalf("max_tokens=%v body=%s", parsed.MaxTokens, out)
	}
	if parsed.MaxCompletionTokens != nil {
		t.Fatalf("openai field leaked: %s", out)
	}
}

func TestSanitize_dropsOpenAIOnlyFields(t *testing.T) {
	in := []byte(`{"model":"mistral-small-latest","stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`)
	out := Sanitize(in)
	if strings.Contains(string(out), "stream_options") {
		t.Fatalf("stream_options leaked: %s", out)
	}
}

func TestSanitize_noopOnAlreadyCleanBody(t *testing.T) {
	in := []byte(`{"model":"mistral-small-latest","messages":[{"role":"user","content":"hi"}]}`)
	out := Sanitize(in)
	if string(out) != string(in) {
		t.Fatalf("expected original bytes, got %s", out)
	}
}
