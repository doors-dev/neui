package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExternalRefRejected(t *testing.T) {
	if _, err := NewRawSchema(json.RawMessage(`{"type":"object","properties":{"a":{"$ref":"file:///etc/hosts"}}}`)); err == nil {
		t.Fatal("file:// ref compiled")
	}
	path := filepath.Join(t.TempDir(), "ref.json")
	if err := os.WriteFile(path, []byte(`{"type":"string"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRawSchema(json.RawMessage(`{"type":"object","properties":{"a":{"$ref":"file://` + path + `"}}}`)); err == nil {
		t.Fatal("file:// ref to a readable schema compiled")
	}
	if _, err := NewRawSchema(json.RawMessage(`{"type":"object","properties":{"a":{"$ref":"http://example.com/s.json"}}}`)); err == nil {
		t.Fatal("http:// ref compiled")
	}
}

func TestLocalRefCompiles(t *testing.T) {
	s, err := NewRawSchema(json.RawMessage(`{"type":"object","properties":{"a":{"$ref":"#/$defs/name"}},"required":["a"],"$defs":{"name":{"type":"string"}}}`))
	if err != nil {
		t.Fatalf("local ref: %v", err)
	}
	if err := s.ValidateString(`{"a":"x"}`); err != nil {
		t.Fatalf("valid instance rejected: %v", err)
	}
	if err := s.ValidateString(`{"a":1}`); err == nil {
		t.Fatal("invalid instance accepted")
	}
}
