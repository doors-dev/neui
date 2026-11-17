package notify

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestKind(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{context.Canceled, ""},
		{fmt.Errorf("rename: %w", context.Canceled), ""},
		{context.DeadlineExceeded, "server"},
		{errors.New("disk full"), "server"},
	}
	for _, c := range cases {
		if got := kind(c.err); got != c.want {
			t.Errorf("kind(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}
