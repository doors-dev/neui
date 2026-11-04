package lib

import (
	"bytes"
	"encoding/json"
)

// MarshalRaw is json.Marshal without HTML escaping: <, > and & stay as they
// are, including inside json.RawMessage values, which json.Marshal would
// rewrite as \u escapes. Raw messages are still compacted. There is no
// trailing newline.
func MarshalRaw(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
