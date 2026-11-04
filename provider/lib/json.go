package lib

import (
	"bytes"
	"encoding/json"
)

// MarshalRaw encodes without rewriting <, > and & into escapes. A replayed
// opaque payload is stored as the provider sent it and spliced into the body
// unparsed, and encoding/json escapes those three bytes inside a RawMessage on
// the way out, which preserves the value and not the bytes. The provider is
// what decides whether that distinction matters, so it is not risked.
func MarshalRaw(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
