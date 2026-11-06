package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/doors-dev/doors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"neui.dev/internal/common"
	"neui.dev/internal/schema"
	"neui.dev/internal/tools"
	"neui.dev/internal/utils"
)

func Module(conf Config) common.ModuleFactory {
	conf.initDefaults()
	conf.Name = utils.SanitizeName(conf.Name)
	return func(ctx context.Context) common.Module {
		m := &module{
			ctx:         ctx,
			conf:        conf,
			ch:          make(chan command),
			initGuard:   make(chan struct{}),
			initTimeout: time.After(5 * time.Second),
		}
		go m.run()
		return m
	}
}

type Config struct {
	Name        string
	URL         string
	Headers     map[string]string
	Enable      func(toolName string) bool
	IdleTimeout time.Duration
	Ping        time.Duration
}

func (c *Config) initDefaults() {
	if c.Enable == nil {
		c.Enable = func(name string) bool {
			return true
		}
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 10 * time.Minute
	}
	if c.Ping == 0 {
		c.Ping = 30 * time.Second
	}
}

type module struct {
	ctx              context.Context
	initGuard        chan struct{}
	initTimeout      <-chan time.Time
	conf             Config
	module           tools.Module
	ch               chan command
	idle             *time.Timer
	client           *mcp.ClientSession
	seq              int
	cancelConnection context.CancelFunc
	toolCalls        int
	inflight         int
	desc             atomic.Pointer[string]
	connErrored      bool
	connectedAt      time.Time
	backoff          backoff
	waiters          []getSessionCommand
	once             sync.Once
}

func (m *module) waitInit() {
	select {
	case <-m.initGuard:
	case <-m.initTimeout:
		m.closeInit()
	case <-m.ctx.Done():
	}
}

func (m *module) closeInit() {
	m.once.Do(func() {
		close(m.initGuard)
	})
}

func (m *module) Name() string {
	return m.conf.Name
}

func (m *module) Desc() string {
	m.waitInit()
	d := m.desc.Load()
	if d == nil {
		return ""
	}
	return *d
}

func (m *module) Get(name string) (common.Tool, bool) {
	type tool struct {
		tool  common.Tool
		found bool
	}
	ch := make(chan tool)
	if !m.send(apiCommand{func(m *module) {
		if m.module == nil {
			ch <- tool{found: false}
			return
		}
		t, ok := m.module.Get(name)
		ch <- tool{t, ok}
	}}) {
		return nil, false
	}
	t := <-ch
	return t.tool, t.found
}

func (m *module) List() []common.Tool {
	m.waitInit()
	ch := make(chan []common.Tool)
	if !m.send(apiCommand{func(m *module) {
		if m.module == nil {
			ch <- nil
			return
		}
		ch <- m.module.List()
	}}) {
		return nil
	}
	return <-ch
}

var _ common.Module = (*module)(nil)

func (m *module) run() {
	m.connect(false)
	defer m.closeConnection()
	for {
		var idle <-chan time.Time
		if m.idle != nil {
			idle = m.idle.C
		}
		select {
		case <-m.ctx.Done():
			return
		case <-idle:
			if m.inflight > 0 || len(m.waiters) > 0 {
				m.touch()
				continue
			}
			m.closeConnection()
			m.idle = nil
		case cmd := <-m.ch:
			m.on(cmd)
		}
	}
}

func (m *module) on(cmd command) {
	switch cmd := cmd.(type) {
	case connectCommand:
		if m.seq != cmd.seq {
			go cmd.client.Close()
			return
		}
		m.closeInit()
		m.cancelConnection = nil
		m.connErrored = false
		m.connectedAt = time.Now()
		m.client = cmd.client
		m.desc.Store(&cmd.desc)
		m.module = m.buildModule(cmd.tools)
		for _, w := range m.waiters {
			if w.reply(cmd.client) {
				m.inflight += 1
			}
		}
		m.waiters = nil
		m.touch()
	case connectErrCommand:
		if m.seq != cmd.seq {
			return
		}
		m.closeInit()
		doors.Logger(m.ctx).Error(
			"mcp: connection failed, retrying",
			"module", m.conf.Name,
			"url", m.conf.URL,
			"err", cmd.err,
		)
		m.connErrored = true
		for _, w := range m.waiters {
			w.reply(nil)
		}
		m.waiters = nil
		m.touch()
	case toolsCommand:
		if m.seq != cmd.seq {
			return
		}
		if cmd.err != nil {
			doors.Logger(m.ctx).Error(
				"mcp: tool list failed, reconnecting",
				"module", m.conf.Name,
				"err", cmd.err,
			)
			m.reconnect()
			return
		}
		m.module = m.buildModule(cmd.tools)
	case callDoneCommand:
		m.inflight -= 1
		m.touch()
	case getSessionCommand:
		m.toolCalls += 1
		switch true {
		case m.client != nil:
			if cmd.reply(m.client) {
				m.inflight += 1
				m.touch()
			}
		case m.isConnecting() && m.connErrored:
			cmd.reply(nil)
		case m.isConnecting():
			m.waiters = append(m.waiters, cmd)
		default:
			m.waiters = append(m.waiters, cmd)
			m.connect(false)
		}
	case apiCommand:
		cmd.f(m)
	case disconnectCommand:
		if m.seq != cmd.seq {
			return
		}
		doors.Logger(m.ctx).Warn(
			"mcp: connection lost, reconnecting",
			"module", m.conf.Name,
		)
		m.reconnect()
	}
}

func (m *module) touch() {
	d := m.conf.IdleTimeout
	if m.toolCalls == 0 {
		d = min(1*time.Minute, d)
	}
	if m.idle == nil {
		m.idle = time.NewTimer(d)
		return
	}
	m.idle.Reset(d)
}

func (m *module) buildModule(list []*mcp.Tool) tools.Module {
	desc := ""
	if d := m.desc.Load(); d != nil {
		desc = *d
	}
	module := tools.NewModule(m.conf.Name, desc)
	counts := make(map[string]int, len(list))
	for _, tool := range list {
		counts[utils.SanitizeName(tool.Name)] += 1
	}
	for _, tool := range list {
		name := utils.SanitizeName(tool.Name)
		if name == "" || counts[name] > 1 {
			sum := sha256.Sum256([]byte(tool.Name))
			if name == "" {
				name = "t" + hex.EncodeToString(sum[:4])
			} else {
				name = name + "_" + hex.EncodeToString(sum[:4])
			}
		}
		inputSchema, err := schema.NewRawSchema(tool.InputSchema)
		if err != nil {
			doors.Logger(m.ctx).Error(
				"mcp: tool skipped, input schema invalid",
				"module", m.conf.Name,
				"tool", tool.Name,
				"err", err,
			)
			continue
		}
		if bytes.Equal(inputSchema.Raw(), schema.AnyRawSchema.Raw()) {
			inputSchema = schema.ObjectRawSchema
		}
		outputSchema := schema.BlocksRawSchema
		structured := false
		if tool.OutputSchema != nil {
			if declared, err := schema.NewRawSchema(tool.OutputSchema); err == nil {
				outputSchema = declared
				structured = true
			}
		}
		module.AddRaw(
			name,
			tool.Description,
			inputSchema,
			outputSchema,
			func(ctx context.Context, input json.RawMessage) (any, error) {
				ch := make(chan *mcp.ClientSession)
				select {
				case <-ctx.Done():
					return nil, context.Cause(ctx)
				case <-m.ctx.Done():
					return nil, context.Cause(m.ctx)
				case m.ch <- getSessionCommand{ctx, ch}:
				}
				var client *mcp.ClientSession
				select {
				case <-ctx.Done():
					return nil, context.Cause(ctx)
				case <-m.ctx.Done():
					return nil, context.Cause(m.ctx)
				case client = <-ch:
				}
				if client == nil {
					return nil, errors.New("the MCP server is unreachable right now; calls fail until it reconnects. Do not retry now; tell the user.")
				}
				defer m.send(callDoneCommand{})
				res, err := client.CallTool(
					ctx,
					&mcp.CallToolParams{
						Name:      tool.Name,
						Arguments: input,
					},
				)
				if err != nil {
					return nil, err
				}
				if res.IsError {
					return nil, resultError(res)
				}
				if res.NeedsInput() {
					return nil, inputRequired(res.InputRequests)
				}
				return result(ctx, res, structured), nil
			},
		)
	}
	return module
}

func (m *module) isConnecting() bool {
	return m.cancelConnection != nil && m.client == nil
}

func (m *module) reconnect() {
	healthy := time.Since(m.connectedAt) >= 30*time.Second
	m.closeConnection()
	if healthy {
		m.backoff.reset()
	}
	m.connect(!healthy)
}

func (m *module) connect(delayed bool) {
	if m.ctx.Err() != nil {
		return
	}
	m.seq += 1
	m.connErrored = delayed
	seq := m.seq
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelConnection = cancel
	go func() {
		if delayed && !m.backoff.wait(ctx) {
			return
		}
		reported := false
		var s *mcp.ClientSession
		var list []*mcp.Tool
		var err error
	retry:
		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		changed := make(chan struct{}, 1)
		s, err = connect(attemptCtx, m.conf.URL, m.conf.Headers, m.conf.Ping, func() {
			select {
			case changed <- struct{}{}:
			default:
			}
		})
		if err == nil {
			if list, err = getTools(attemptCtx, m.conf.Enable, s); err != nil {
				s.Close()
			}
		}
		cancel()
		if err != nil {
			if !reported {
				reported = true
				if !m.send(connectErrCommand{err, seq}) {
					return
				}
			}
			if !m.backoff.wait(ctx) {
				return
			}
			goto retry
		}
		if !m.send(connectCommand{s, getDesc(s), list, seq}) {
			s.Close()
			return
		}
		closed := make(chan struct{})
		go func() {
			for {
				select {
				case <-closed:
					return
				case <-changed:
					list, err := getTools(ctx, m.conf.Enable, s)
					m.send(toolsCommand{list, err, seq})
				}
			}
		}()
		s.Wait()
		close(closed)
		m.send(disconnectCommand{seq})
	}()
}

func (m *module) closeConnection() {
	m.seq += 1
	if m.cancelConnection != nil {
		m.cancelConnection()
		m.cancelConnection = nil
	}
	if m.client != nil {
		go m.client.Close()
		m.client = nil
	}
}

func (m *module) send(c command) bool {
	select {
	case m.ch <- c:
		return true
	case <-m.ctx.Done():
		return false
	}
}

func getTools(ctx context.Context, enable func(string) bool, s *mcp.ClientSession) ([]*mcp.Tool, error) {
	a := make([]*mcp.Tool, 0)
	for t, e := range s.Tools(ctx, &mcp.ListToolsParams{}) {
		if e != nil {
			return nil, e
		}
		if !enable(t.Name) {
			continue
		}
		a = append(a, t)
	}
	return a, nil
}
func getDesc(s *mcp.ClientSession) string {
	init := s.InitializeResult()
	desc := init.Instructions
	if init.ServerInfo != nil && init.ServerInfo.Description != "" {
		if desc == "" {
			return init.ServerInfo.Description
		}
		desc = init.ServerInfo.Description + "\n\n" + desc
	}
	return desc
}
