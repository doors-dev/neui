package chat

import (
	"context"
	"testing"
)

func TestNameToolRejectsLineBreaksAndControls(t *testing.T) {
	for _, name := range []string{
		"a\nb", "a\rb", "a\u2028b", "a\u2029b", "a\u0085b", "a\vb", "a\fb", "a\tb", "a\x00b", "a\x1bb",
	} {
		_, err := nameTool(context.Background(), NameArg{Name: name})
		if err == nil || err.Error() != "name must be a single line" {
			t.Errorf("%q: got %v, want single line error", name, err)
		}
	}
}
