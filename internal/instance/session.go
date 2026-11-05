package instance

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/instance/ui"
	chatmod "neui.dev/internal/modules/chat"
	"neui.dev/internal/modules/files"
	timemod "neui.dev/internal/modules/time"
	"neui.dev/internal/page"
	"neui.dev/internal/storage/local"
)

func GetSession(ctx context.Context) Session {
	return doors.SessionStore(ctx).Init(common.KeySession, func() any {
		return &session{
			ctx:                 doors.SessionContext(ctx),
			localMessageStorage: local.NewMessageStorage(),
			instances:           make(map[*instance]struct{}),
			chats:               ui.NewChats(),
		}
	}).(Session)
}

type Session = *session

type session struct {
	mu                  sync.Mutex
	ctx                 context.Context
	localMessageStorage api.MessageStorage
	localFileStorage    *local.FileStorage
	backend             *common.Backend
	cancel              context.CancelFunc
	instances           map[*instance]struct{}
	chats               ui.Chats
	zone                atomic.Pointer[time.Location]
}

func (s Session) Chats() doors.Beam[map[api.ThreadID]api.InstanceChat] {
	return s.chats.Beam()
}

func (s Session) Location() *time.Location {
	if zone := s.zone.Load(); zone != nil {
		return zone
	}
	return time.Local
}

func (s Session) removeInstance(inst *instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.instances, inst)
	s.chats.Remove(context.Background(), inst.id)
}

func (s Session) newInstance(id string, ctx context.Context) Instance {
	inst := &instance{
		id:      id,
		ctx:     ctx,
		session: s,
		ch:      make(chan command),
		links:   make(map[*link]struct{}),
		dead:    make(map[*link]struct{}),
		page:    page.NewPage(ctx),
		current: doors.TabState[api.ThreadID](ctx, "neui_thread"),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	inst.backend.Store(s.backend)
	status := api.InstanceDisabled
	if s.backend != nil {
		status = api.InstanceLoading
	}
	inst.state = s.chats.Add(ctx, id, api.InstanceState{Status: status})
	s.instances[inst] = struct{}{}
	go inst.run()
	return inst
}

func (s Session) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backend == nil {
		return
	}
	s.backend = nil
	for inst := range maps.Keys(s.instances) {
		inst.disable()
	}

}

func (s Session) Enable(opts []common.Option) error {
	forced := common.BackendForced(opts)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backend != nil && !forced {
		return nil
	}
	if s.localFileStorage == nil {
		fs, err := local.NewFileStorage(common.NewBackend(s.ctx, nil, nil, nil, opts).FilesConfig().TempDir)
		if err != nil {
			return fmt.Errorf("instance: local file storage: %w", err)
		}
		s.localFileStorage = fs
	}
	options := make([]common.Option, 0, len(opts)+5)
	options = append(options, common.OptionModule(chatmod.Module))
	options = append(options, common.OptionModule(timemod.Module))
	options = append(options, common.OptionModule(files.Module))
	options = append(options, common.OptionModules(common.AppModules(s.ctx)))
	options = append(options, common.OptionSystem(common.AppSystem(s.ctx)))
	options = append(options, opts...)
	b := common.NewBackend(s.ctx, s.localMessageStorage, s.localFileStorage, s.localFileStorage, options)
	if b.Provider() == nil {
		return api.ErrNoProvider
	}
	s.backend = b
	for inst := range maps.Keys(s.instances) {
		inst.enable(b)
	}
	return nil
}
