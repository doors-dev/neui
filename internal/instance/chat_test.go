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

func TestChatMainCancelsALinkDroppedBeforeInit(t *testing.T) {
	for _, reload := range []bool{true, false} {
		instCtx, instCancel := context.WithCancel(context.Background())
		inst := &instance{ctx: instCtx, ch: make(chan command), links: make(map[*link]struct{})}
		first := make(chan Link, 1)
		go func() {
			for {
				select {
				case <-instCtx.Done():
					return
				case cmd := <-inst.ch:
					if cmd, ok := cmd.(commandLink); ok && len(first) == 0 && cap(first) == 1 {
						first <- cmd.link
						if reload {
							cmd.link.send(thread.NewMessageReload(nil))
						}
						cmd.link.disconnect()
						first = make(chan Link)
					}
				}
			}
		}()
		got := first
		renderCtx, renderCancel := context.WithCancel(context.Background())
		c := &chat{inst: inst}
		done := make(chan error, 1)
		go func() {
			done <- c.main()(gox.NewCursor(renderCtx, gox.NewPrinter(io.Discard)))
		}()
		var l Link
		select {
		case l = <-got:
		case <-time.After(5 * time.Second):
			t.Fatal("the render never took a link")
		}
		if !reload {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("the render did not return on a closed link")
			}
		}
		select {
		case <-l.ctx.Done():
		case <-time.After(5 * time.Second):
			t.Fatalf("reload=%v: the dropped link's context was never cancelled", reload)
		}
		renderCancel()
		instCancel()
	}
}
