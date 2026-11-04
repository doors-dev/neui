package instance

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/thread"
)

func TestChatMainClosesTheLinkOnADeadRenderContext(t *testing.T) {
	instCtx, instCancel := context.WithCancel(context.Background())
	defer instCancel()
	inst := &instance{ctx: instCtx, ch: make(chan command), links: make(map[*link]struct{})}
	linked, unlinked := make(chan Link, 1), make(chan Link, 1)
	go func() {
		for {
			select {
			case <-instCtx.Done():
				return
			case cmd := <-inst.ch:
				switch cmd := cmd.(type) {
				case commandLink:
					inst.links[cmd.link] = struct{}{}
					linked <- cmd.link
				case commandUnlink:
					delete(inst.links, cmd.link)
					unlinked <- cmd.link
				}
			}
		}
	}()

	renderCtx, renderCancel := context.WithCancel(context.Background())
	c := &chat{inst: inst}
	done := make(chan error, 1)
	go func() {
		done <- c.main()(gox.NewCursor(renderCtx, gox.NewPrinter(io.Discard)))
	}()
	var l Link
	select {
	case l = <-linked:
	case <-time.After(5 * time.Second):
		t.Fatal("the render never took a link")
	}
	renderCancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the render returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the render did not return on the dead context")
	}
	select {
	case got := <-unlinked:
		if got != l {
			t.Fatalf("a different link was closed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the abandoned link was never closed: it stays in inst.links uninitialized")
	}

	sent := make(chan struct{})
	go func() {
		l.send(thread.NewMessageInit(api.ChatState{}, nil))
		close(sent)
	}()
	select {
	case <-sent:
	case <-time.After(5 * time.Second):
		t.Fatal("send to the abandoned link blocked: the next broadcast parks the instance command loop")
	}
}
