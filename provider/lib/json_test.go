package lib

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMarshalRawKeepsHTMLBytes is the whole reason MarshalRaw exists. A payload
// a provider signed goes back inside the body as the bytes it arrived as, and
// encoding/json rewrites <, > and & into \u00XX escapes inside a RawMessage
// unless it is told not to, which preserves the value and not the bytes.
func TestMarshalRawKeepsHTMLBytes(t *testing.T) {
	payload := json.RawMessage(`{"code":"if a<b && c>d {}"}`)
	body := struct {
		Item json.RawMessage `json:"item"`
	}{payload}
	raw, err := MarshalRaw(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), string(payload)) {
		t.Fatalf("the payload was rewritten:\n got: %s\nwant to contain: %s", raw, payload)
	}
	plain, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plain), "\\u003c") {
		t.Fatalf("encoding/json stopped escaping the three bytes, so MarshalRaw has nothing left to do: %s", plain)
	}
}

// TestMarshalRawCompactsWhitespace pins the one difference that survives. The
// encoder runs every RawMessage through compact, which drops the whitespace
// between tokens, so the bytes are the payload's own for everything a signature
// covers and not for how it was laid out.
func TestMarshalRawCompactsWhitespace(t *testing.T) {
	raw, err := MarshalRaw(json.RawMessage("{\"a\": 1,\n \"b\": 2}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"a":1,"b":2}` {
		t.Fatalf("got %s, want %s", raw, `{"a":1,"b":2}`)
	}
}
