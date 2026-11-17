package chat

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
)

func TestClosedKindOf(t *testing.T) {
	cases := []struct {
		err  error
		want closedKind
	}{
		{api.ErrNotFound, deleted},
		{fmt.Errorf("thread: %w", api.ErrNotFound), deleted},
		{api.ErrCompactionFailed, tooLong},
		{fmt.Errorf("thread: %w", api.ErrCompactionFailed), tooLong},
		{errors.New("disk full"), failed},
		{nil, failed},
	}
	for _, c := range cases {
		if got := closedKindOf(c.err); got != c.want {
			t.Errorf("closedKindOf(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

type reloadThread struct {
	api.ChatThread
}

func (reloadThread) Reload(context.Context) {}

func TestTooLongChatShowsItsOwnCard(t *testing.T) {
	c := &chat{thread: reloadThread{}}
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Elem {
		return c.body(closedKindOf(api.ErrCompactionFailed))
	})
	srv := httptest.NewServer(app)
	defer srv.Close()
	res, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	page := html.UnescapeString(string(body))
	for _, want := range []string{"This chat is too long", "It no longer fits the model's context window. Start a new chat to continue.", "nechat-closed-card", ">OK<"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the closed card misses %q: %s", want, page)
		}
	}
	if strings.Contains(page, "Something went wrong") {
		t.Fatalf("the card for a failed compaction shows the generic error: %s", page)
	}
}
