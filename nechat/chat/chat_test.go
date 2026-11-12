package chat

import (
	"errors"
	"fmt"
	"testing"

	"neui.dev/api"
)

func TestClosedKindOf(t *testing.T) {
	cases := []struct {
		err  error
		want closedKind
	}{
		{api.ErrNotFound, deleted},
		{fmt.Errorf("thread: %w", api.ErrNotFound), deleted},
		{errors.New("disk full"), failed},
		{nil, failed},
	}
	for _, c := range cases {
		if got := closedKindOf(c.err); got != c.want {
			t.Errorf("closedKindOf(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
