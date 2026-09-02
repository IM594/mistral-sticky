package compat

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Top-level fields documented on POST /v1/chat/completions.
var keep = map[string]struct{}{
	"frequency_penalty":    {},
	"guardrails":           {},
	"max_tokens":           {},
	"messages":             {},
	"model":                {},
	"n":                    {},
	"parallel_tool_calls":  {},
	"prediction":           {},
	"presence_penalty":     {},
	"prompt_cache_key":     {},
	"prompt_mode":          {},
	"random_seed":          {},
	"reasoning_effort":     {},
	"response_format":      {},
	"safe_prompt":          {},
	"service_tier":         {},
	"stop":                 {},
	"stream":               {},
	"temperature":          {},
	"tool_choice":          {},
	"tools":                {},
	"top_p":                {},
}

// Sanitize drops OpenAI-only fields and clamps reasoning_effort to values
// this Mistral deployment accepts (high | none).
func Sanitize(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return body
	}
	changed := false

	if _, has := root["max_tokens"]; !has {
		if v, ok := root["max_completion_tokens"]; ok {
			root["max_tokens"] = v
			changed = true
		}
	}
	if _, has := root["random_seed"]; !has {
		if v, ok := root["seed"]; ok {
			root["random_seed"] = v
			changed = true
		}
	}
	if raw, ok := root["reasoning_effort"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			switch strings.ToLower(strings.TrimSpace(s)) {
			case "high", "none":
			default:
				delete(root, "reasoning_effort")
				changed = true
			}
		} else {
			delete(root, "reasoning_effort")
			changed = true
		}
	}
	for k := range root {
		if _, ok := keep[k]; ok {
			continue
		}
		delete(root, k)
		changed = true
	}
	if !changed {
		return body
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return body
	}
	return bytes.TrimSpace(buf.Bytes())
}
