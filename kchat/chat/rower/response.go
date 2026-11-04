package rower

import (
	"context"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/kchat/chat/rower/marksman"
	"neui.dev/kchat/params"
)

func NewResponse(p params.Params, state api.Indexed[api.Content] /* results map[string]api.ContentToolResult, */, fs api.FS) Response {
	content := make(map[int]streamer[api.IndexedEvent])
	for index, c := range state {
		switch c := c.(type) {
		case api.ContentText:
			content[index] = newTextRow(c.Text, fs)
		case api.ContentReasoning:
			if p.Off(params.Thinking) {
				continue
			}
			content[index] = newThinking(c.Thoughts, c.Summary)
		}
	}
	r := newRower(content, func(e api.IndexedEvent) streamer[api.IndexedEvent] {
		switch e := e.(type) {
		case api.EventText:
			return newTextRow(e.Text, fs)
		case api.EventThinking:
			if p.Off(params.Thinking) {
				return nil
			}
			return newThinking(
				map[int]string{
					e.Part: e.Text,
				},
				nil,
			)
		case api.EventSummary:
			if p.Off(params.Thinking) {
				return nil
			}
			return newThinking(
				nil,
				map[int]string{
					e.Part: e.Text,
				},
			)
		default:
			return nil
		}
	})
	return &response{
		rower: r,
		//		tools: NewTools(state, results != nil),
	}
}

type Response = *response

type response struct {
	rower *rower[api.IndexedEvent]
	// tools Tools
}

func (r Response) IsEmpty() bool {
	return r.rower.IsEmpty()
}

func (r Response) Close() {
	r.rower.Close()
	// r.tools.Close()
}

/*
func (r Response) Results(results map[string]api.ContentToolResult) {
	r.tools.Done()
} */

func (r Response) On(e api.IndexedEvent) {
	switch e := e.(type) {
	case api.EventText:
		r.rower.Append(e)
	case api.EventThinking:
		r.rower.Append(e)
	case api.EventSummary:
		r.rower.Append(e)
	/*
		case api.EventTool:
			r.tools.Append(e) */
	case api.EventClose:
		r.rower.Append(e)
	default:
	}
}

func (r Response) Main() gox.Elem {
	return r.rower.Main()
	/*
		return func(cur gox.Cursor) error {
			if err := cur.Comp(r.rower); err != nil {
				return err
			}
			return cur.Comp(r.tools)
		} */
}

func newTextRow(text string, fs api.FS) *textRow {
	return &textRow{
		Marksman: marksman.NewMarksman(text, 5*time.Second, fs),
		slot:     newSlot(textRunning()),
	}
}

type textRow struct {
	marksman.Marksman
	slot   *doors.Door
	closed bool
}

var _ streamer[api.IndexedEvent] = &textRow{}

func (t *textRow) Main() gox.Elem {
	return block{"Text", t.slot, gox.Elem(func(cur gox.Cursor) error {
		if err := cur.Init("div"); err != nil {
			return err
		}
		if err := cur.Set("class", "kchat-col kchat-prose"); err != nil {
			return err
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Comp(t.Marksman); err != nil {
			return err
		}
		return cur.Close()
	})}.Main()
}

func (t *textRow) Append(e api.IndexedEvent) {
	switch e := e.(type) {
	case api.EventText:
		t.Marksman.Append(e.Text)
	case api.EventClose:
		t.Close()
	}
}

func (t *textRow) Close() {
	if t.closed {
		return
	}
	t.closed = true
	t.Marksman.Close()
	t.slot.Static(context.Background(), textDone())
}
