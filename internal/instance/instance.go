package instance

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/doors-dev/doors"
	"github.com/gammazero/deque"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/instance/ui"
	"neui.dev/internal/page"
	"neui.dev/internal/thread"
)

func GetInstance(ctx context.Context) Instance {
	return doors.InstanceStore(ctx).Init(common.KeyInstance, func() any {
		sess := GetSession(ctx)
		return sess.newInstance(doors.InstanceID(ctx), doors.InstanceContext(ctx))
	}).(Instance)
}

func GetSpace(ctx context.Context) page.Space {
	space, ok := ctx.Value(common.KeySpace).(page.Space)
	if ok {
		return space
	}
	return GetInstance(ctx).getSpace()
}

type Instance = *instance

type instance struct {
	id      string
	ctx     context.Context
	session *session
	uplink  thread.Link
	queue   deque.Deque[thread.Command]
	state   ui.State
	backend atomic.Pointer[common.Backend]
	ch      chan command
	links   map[*link]struct{}
	dead    map[*link]struct{}
	page    page.Page
	current doors.Source[*api.ThreadID]
	synced  doors.Source[bool]
}

func (inst *instance) enable(b *common.Backend) {
	inst.send(commandSwitch{b})
}

func (inst *instance) disable() {
	inst.send(commandSwitch{nil})
}

func (inst *instance) getSpace() page.Space {
	return inst.page.Space()
}

func (inst *instance) Synced() doors.Beam[bool] {
	return inst.synced
}

func (inst Instance) link() (Link, bool) {
	link := newLink(inst)
	if inst.send(commandLink{link}) != nil {
		return nil, false
	}
	return link, true
}

var _ Instance = (*instance)(nil)

func (inst *instance) run() {
	defer inst.destruct()
	tab := make(chan struct{})
	inst.current.Sub(inst.ctx, func(ctx context.Context, ti *api.ThreadID) bool {
		if ti == nil {
			return false
		}
		close(tab)
		return true
	})
	synced := make(chan struct{})
	go func() {
		if inst.session.zone.Load() == nil {
			var now time.Time
			ch := doors.Call(inst.ctx, doors.ActionTime{}.Into(&now))
			if err := <-ch; err == nil {
				if err, ok := <-ch; ok && err == nil {
					inst.session.zone.Store(now.Location())
				}
			}
		}
		select {
		case <-tab:
			close(synced)
		case <-inst.ctx.Done():
		}
	}()
	for {
		var recv <-chan thread.Message
		var done <-chan struct{}
		var send chan<- thread.Command
		var front thread.Command
		if inst.uplink.Valid() {
			recv, done = inst.uplink.Receiver()
			if inst.queue.Len() > 0 {
				send, front = inst.uplink.Sender(), inst.queue.Front()
			}
		}
		select {
		case <-inst.ctx.Done():
			return
		case cmd := <-inst.ch:
			inst.onCommand(cmd)
		case <-done:
			inst.uplink = thread.Link{}
			inst.renewUplink()
		case m := <-recv:
			inst.onMessage(m)
		case send <- front:
			inst.queue.PopFront()
		case <-synced:
			synced = nil
			inst.synced.Update(context.Background(), true)
			if inst.backend.Load() != nil {
				inst.getUplink()
			}
		}
	}
}

func (inst *instance) onCommand(cmd command) {
	switch cmd := cmd.(type) {
	case commandSetUplink:
		if !inst.synced.Get() || inst.backend.Load() != cmd.backend {
			cmd.uplink.Close()
			cmd.Err(api.ErrUnlinked)
			return
		}
		inst.uplink.Close()
		inst.setUplink(cmd.ctx, cmd.uplink)
		inst.reloadLinks(thread.NewMessageReload(cmd.Operation))
	case commandDrop:
		if !inst.synced.Get() {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if !inst.uplink.Valid() || inst.uplink.ID() != cmd.id {
			cmd.NoMessage()
			return
		}
		inst.uplink.Close()
		inst.setUplink(cmd.ctx, thread.NewLink(inst.backend.Load(), inst.page))
		inst.reloadLinks(thread.NewMessageReload(cmd.Operation))
	case commandLink:
		inst.links[cmd.link] = struct{}{}
		if inst.synced.Get() && inst.backend.Load() != nil {
			cmd.link.backend = inst.backend.Load()
			if inst.uplink.Valid() {
				cmd.link.thread = inst.uplink.ID()
				inst.sendUplink(inst.uplink.Init())
				return
			}
		}
		cmd.link.send(thread.NewMessageInit(api.ChatState{}, nil))
	case commandUnlink:
		delete(inst.links, cmd.link)
		delete(inst.dead, cmd.link)
	case commandSwitch:
		if inst.backend.Load() == nil && cmd.backend == nil {
			return
		}
		inst.backend.Store(cmd.backend)
		if inst.uplink.Valid() {
			inst.uplink.Close()
			inst.uplink = thread.Link{}
		}
		if cmd.backend == nil {
			inst.state.Disable(context.Background())
			if inst.synced.Get() {
				inst.reloadLinks(thread.NewMessageReload(nil))
			}
			return
		}
		if inst.synced.Get() {
			inst.getUplink()
			return
		}
		inst.state.Enable(context.Background())
	case commandUplink:
		if _, ok := inst.links[cmd.link]; !ok {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if !inst.uplink.Valid() {
			cmd.Err(api.ErrUnlinked)
			return
		}
		inst.sendUplink(cmd.makeCommand(inst.uplink))
	case commandRecord:
		if !inst.synced.Get() || !inst.uplink.Valid() {
			cmd.Err(api.ErrUnlinked)
			return
		}
		inst.sendUplink(inst.uplink.Record(cmd.Operation, cmd.kind, cmd.attrs, cmd.data))
	default:
		panic("unknown command")
	}
}

func (inst *instance) sendUplink(cmd thread.Command) {
	inst.queue.PushBack(cmd)
}

func (inst *instance) getUplink() {
	id := *inst.current.Get()
	if id == "" {
		inst.renewUplink()
		return
	}
	if _, err := inst.backend.Load().Storage().ThreadGet(inst.ctx, id); err != nil {
		inst.renewUplink()
		return
	}
	uplink, err := thread.OpenLink(id, inst.backend.Load(), inst.page, nil, nil)
	if err != nil {
		inst.renewUplink()
		return
	}
	inst.setUplink(context.Background(), uplink)
	inst.reloadLinks(thread.NewMessageReload(nil))
}

func (inst *instance) setUplink(ctx context.Context, uplink thread.Link) {
	inst.uplink = uplink
	id := inst.uplink.ID()
	inst.state.Open(ctx, uplink.Thread())
	inst.current.Update(inst.ctx, &id)
}

func (inst *instance) renewUplink() {
	inst.uplink.Close()
	uplink := thread.NewLink(inst.backend.Load(), inst.page)
	inst.setUplink(context.Background(), uplink)
	inst.reloadLinks(thread.NewMessageReload(nil))
}

func (inst *instance) reloadLinks(m thread.MessageReload) {
	defer m.Ack()
	inst.clearQueue()
	for link := range inst.links {
		link.send(m)
		link.disconnect()
	}
	clear(inst.links)
	for link := range inst.dead {
		link.send(m)
		link.disconnect()
	}
	clear(inst.dead)
}

func (inst *instance) clearQueue() {
	for cmd := range inst.queue.IterPopFront() {
		if e, ok := cmd.(thread.Errorable); ok {
			e.Err(api.ErrUnlinked)
		}
	}
}

func (inst *instance) onMessage(m thread.Message) {
	if r, ok := m.(thread.MessageReload); ok {
		inst.reloadLinks(r)
		return
	}
	defer m.Ack()
	if m, ok := m.(thread.MessageEvent); ok {
		switch e := m.Event.(type) {
		case api.ChatEventCriticalError:
			defer inst.onCriticalErr()
		case api.ChatEventName:
			inst.state.Rename(context.Background(), e.Name)
		case api.ChatEventThreadCreated:
			inst.state.Create(context.Background())
		case api.ChatEventRoundStart:
			inst.state.Touch(context.Background(), time.Now())
		}
	}
	for link := range inst.links {
		link.send(m)
	}
}

func (inst *instance) onCriticalErr() {
	for link := range inst.links {
		link.dead.Store(true)
		if link.initialized {
			inst.dead[link] = struct{}{}
			continue
		}
		link.send(thread.NewMessageReload(nil))
		link.disconnect()
	}
	clear(inst.links)
	inst.clearQueue()
	uplink, err := thread.OpenLink(inst.uplink.ID(), inst.backend.Load(), inst.page, nil, nil)
	if err != nil {
		inst.uplink.Close()
		inst.setUplink(context.Background(), thread.NewLink(inst.backend.Load(), inst.page))
		return
	}
	inst.uplink = uplink
}

func (inst *instance) send(cmd command) error {
	select {
	case inst.ch <- cmd:
		return nil
	case <-inst.ctx.Done():
		return context.Canceled
	}
}

func (inst *instance) destruct() {
	inst.session.removeInstance(inst)
	inst.clearQueue()
	if inst.uplink.Valid() {
		inst.uplink.Close()
		inst.uplink = thread.Link{}
	}
	for link, _ := range inst.links {
		link.disconnect()
	}
	for link := range inst.dead {
		link.disconnect()
	}
}
