// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package instance

import (
	"context"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/thread"
)

type chat struct {
	factory api.ChatFactory
	inst    *instance
	door    *doors.Door
}

func (c chat) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		c.door.Outer(cur.Context(), c.main())
		return cur.Comp(c.door)
	}
}

type controller struct {
	*link
	fs   api.FS
	door *doors.Door
}

func (c controller) Reload(ctx context.Context) {
	c.door.Reload(ctx)
}

func (c controller) FS() api.FS {
	return c.fs
}

func (c *chat) main() gox.Elem {
	return func(cur gox.Cursor) error {
	retry:
		if cur.Context().Err() != nil {
			return nil
		}
		link, ok := c.inst.link()
		if !ok {
			return nil
		}
		var m thread.Message
		select {
		case <-cur.Context().Done():
			link.Close()
			return nil
		case m, ok = <-link.Ch():
			if !ok {
				link.cancel()
				return nil
			}
		}
		switch m := m.(type) {
		case thread.MessageReload:
			m.Ack()
			link.cancel()
			goto retry
		case thread.MessageInit:
			m.Ack()
			if !m.Enabled() {
				go c.loop(nil, link, doors.DetachedContext(cur.Context()))
				return c.factory.Placeholder(c.inst.state.Enabled())(cur)
			}
			chat := c.factory.Chat(controller{link, m.FS, c.door}, m.State)
			doors.OnReady(cur.Context(), func(ctx context.Context) {
				go c.loop(chat, link, ctx)
			})
			doors.OnClean(cur.Context(), func() {
				link.Close()
			})
			return cur.Comp(chat)
		default:
			panic("first message must be init or reload")
		}
	}
}

func (c *chat) loop(chat api.Chat, l *link, ctx context.Context) {
	defer l.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case m, ok := <-l.Ch():
			if !ok {
				return
			}
			c.on(chat, ctx, m)
		}
	}
}

func (c *chat) on(chat api.Chat, ctx context.Context, m thread.Message) {
	switch m := m.(type) {
	case thread.MessageReload:
		doors.OnSettle(ctx, func(ctx context.Context) {
			m.Ack()
		}, func(ctx context.Context) {
			doors.OnBatch(ctx, func(context.Context) {}, func(ctx context.Context) {
				c.door.Reload(ctx)
			})
		})
	case thread.MessageInit:
		panic("init can't get here")
	case thread.MessageEvent:
		if chat == nil {
			panic("disabled chat cant get event")
		}
		doors.OnSettle(ctx, func(ctx context.Context) {
			m.Ack()
		}, func(ctx context.Context) {
			doors.OnBatch(ctx, func(context.Context) {}, func(ctx context.Context) {
				chat.Update(ctx, m.Event)
			})
		})
	}
}
