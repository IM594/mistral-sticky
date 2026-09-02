package session

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

type chatBody struct {
	Model          string          `json:"model"`
	ConversationID string          `json:"conversation_id"`
	PromptCacheKey string          `json:"prompt_cache_key"`
	Metadata       json.RawMessage `json:"metadata"`
	Messages       []chatMessage   `json:"messages"`
}

type chatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type metadata struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
}

// Key extracts a sticky session identifier from headers or the chat body.
func Key(headers http.Header, body []byte) string {
	var parsed chatBody
	if err := json.Unmarshal(body, &parsed); err == nil {
		var meta metadata
		if len(parsed.Metadata) > 0 {
			_ = json.Unmarshal(parsed.Metadata, &meta)
		}
		if s := strings.TrimSpace(meta.SessionID); s != "" {
			return s
		}
		if s := strings.TrimSpace(meta.UserID); s != "" {
			return s
		}
		if s := strings.TrimSpace(parsed.ConversationID); s != "" {
			return s
		}
		if s := strings.TrimSpace(parsed.PromptCacheKey); s != "" {
			return s
		}
	}
	if headers != nil {
		if s := strings.TrimSpace(headers.Get("X-Session-ID")); s != "" {
			return s
		}
		if s := strings.TrimSpace(headers.Get("Session-Id")); s != "" {
			return s
		}
	}
	return messageHash(parsed)
}

func messageHash(parsed chatBody) string {
	system := firstText(parsed.Messages, "system")
	user := firstText(parsed.Messages, "user")
	sum := sha256.Sum256([]byte(parsed.Model + "\n" + system + "\n" + user))
	return hex.EncodeToString(sum[:])
}

func firstText(messages []chatMessage, role string) string {
	for _, m := range messages {
		if !strings.EqualFold(m.Role, role) {
			continue
		}
		return contentText(m.Content)
	}
	return ""
}

func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Type == "text" || p.Type == "" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	return ""
}

// Fingerprint returns a short, non-reversible log token for a session key.
func Fingerprint(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:12]
}

// InjectPromptCacheKey writes a stable prompt_cache_key without re-encoding messages.
func InjectPromptCacheKey(body []byte, key string) []byte {
	if key == "" || len(body) == 0 {
		return body
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return body
	}
	enc, err := json.Marshal(key)
	if err != nil {
		return body
	}
	if existing, ok := root["prompt_cache_key"]; ok && bytes.Equal(existing, enc) {
		return body
	}
	root["prompt_cache_key"] = enc
	var buf bytes.Buffer
	encJSON := json.NewEncoder(&buf)
	encJSON.SetEscapeHTML(false)
	if err := encJSON.Encode(root); err != nil {
		return body
	}
	return bytes.TrimSpace(buf.Bytes())
}
