package toolid

import (
	"crypto/sha256"
	"encoding/json"
	"regexp"
)

var validID = regexp.MustCompile(`^[a-zA-Z0-9]{9}$`)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func MapID(original string) string {
	if validID.MatchString(original) {
		return original
	}
	sum := sha256.Sum256([]byte(original))
	out := make([]byte, 9)
	for i := 0; i < 9; i++ {
		out[i] = alphabet[int(sum[i])%len(alphabet)]
	}
	return string(out)
}

func RewriteBody(body []byte) ([]byte, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return body, nil
	}
	rawMsgs, ok := root["messages"]
	if !ok {
		return body, nil
	}
	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(rawMsgs, &messages); err != nil {
		return body, nil
	}
	changed := false
	idMap := map[string]string{}
	for _, msg := range messages {
		if raw, ok := msg["tool_calls"]; ok {
			var calls []map[string]json.RawMessage
			if err := json.Unmarshal(raw, &calls); err == nil {
				for _, call := range calls {
					if idRaw, ok := call["id"]; ok {
						var id string
						if err := json.Unmarshal(idRaw, &id); err == nil && id != "" {
							mapped := mapCached(idMap, id)
							if mapped != id {
								changed = true
								enc, _ := json.Marshal(mapped)
								call["id"] = enc
							}
						}
					}
				}
				enc, err := json.Marshal(calls)
				if err == nil {
					msg["tool_calls"] = enc
				}
			}
		}
		if raw, ok := msg["tool_call_id"]; ok {
			var id string
			if err := json.Unmarshal(raw, &id); err == nil && id != "" {
				mapped := mapCached(idMap, id)
				if mapped != id {
					changed = true
					enc, _ := json.Marshal(mapped)
					msg["tool_call_id"] = enc
				}
			}
		}
	}
	if !changed {
		return body, nil
	}
	enc, err := json.Marshal(messages)
	if err != nil {
		return body, err
	}
	root["messages"] = enc
	return json.Marshal(root)
}

func mapCached(idMap map[string]string, id string) string {
	if mapped, ok := idMap[id]; ok {
		return mapped
	}
	mapped := MapID(id)
	idMap[id] = mapped
	return mapped
}
