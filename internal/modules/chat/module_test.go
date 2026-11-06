package chat

import (
	"context"
	"testing"
)

func TestNameToolRejectsLineBreaksAndControls(t *testing.T) {
	for _, name := range []string{
		"a\nb", "a\rb", "a\u2028b", "a\u2029b", "a\u0085b", "a\vb", "a\fb", "a\tb", "a\x00b", "a\x1bb",
		"a\u200bb", "a\u200db", "a\ufeffb", "a\u202eb", "a\u2066b", "\u200b",
	} {
		_, err := nameTool(context.Background(), NameArg{Name: name})
		if err == nil || err.Error() != "name must be plain text on a single line" {
			t.Errorf("%q: got %v, want single line error", name, err)
		}
	}
}

func TestNameToolRejectsEmptyButCheckNameAccepts(t *testing.T) {
	for _, name := range []string{"", "  ", "\t "} {
		if _, err := nameTool(context.Background(), NameArg{Name: name}); err == nil || err.Error() != "name is empty: pass a short title naming the topic" {
			t.Errorf("%q: nameTool got %v, want the empty error", name, err)
		}
		if got, err := CheckName(name); err != nil || got != "" {
			t.Errorf("%q: CheckName got %q, %v, want an empty name", name, got, err)
		}
	}
}
