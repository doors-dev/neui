package js

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gammazero/deque"
	"modernc.org/quickjs"
	"neui.dev/internal/modules/js/task"
)

type ConfigKey struct{}

type Seed struct {
	id     *byte
	config Config
}

func NewSeed(c Config) Seed {
	return Seed{id: new(byte), config: c}
}

var defaultSeed = Seed{id: new(byte)}

var runtimes sync.Map

func getRuntime(ctx context.Context) *runtime {
	seed, ok := ctx.Value(ConfigKey{}).(Seed)
	if !ok {
		seed = defaultSeed
	}
	if rt, ok := runtimes.Load(seed.id); ok {
		return rt.(*runtime)
	}
	config := seed.config
	config.initDefaults()
	rt, _ := runtimes.LoadOrStore(seed.id, &runtime{
		config:  config,
		tasks:   make(chan task.WaitingTask),
		reports: make(chan int),
	})
	return rt.(*runtime)
}

// Config tunes the js runtime; a zero field takes its default. The limits
// are shared by every session of an app that sets WithJSConfig, and by every
// app in the process that does not: a script waits to start until a
// worker and enough of TotalMemory are free.
type Config struct {
	// TotalMemory is the sum in bytes of the memory budgets of all scripts
	// executing at once across the app. Default 512 MiB; a value below
	// WorkerMemory is raised to it.
	TotalMemory int
	// WorkerMemory is the largest memory budget in bytes one script may
	// take. Default 128 MiB; a value below DefaultMemory is raised to it.
	WorkerMemory int
	// DefaultMemory is the memory budget in bytes of a script that asks for
	// less or none. Default 16 MiB, at least 4 MiB.
	DefaultMemory int
	// MaxWorkers is how many scripts may execute at once across the app.
	// Default 16.
	MaxWorkers int
	// JobTimeout caps how long one script may execute, its tool calls
	// included: tool calls in flight are cancelled at the limit. A script may
	// ask for less. Default 30s.
	JobTimeout time.Duration
	// AsyncAfter is how long a run keeps the tool call waiting before it
	// detaches and delivers its result later as a background task. Default
	// 10s.
	AsyncAfter time.Duration
	// MaxQueue is how many scripts may wait to start across the app; one
	// submitted past it is rejected. Default 100.
	MaxQueue int
	// MaxCallDepth is how many nested JS calls a script may make before an
	// error is thrown. Default 1000.
	MaxCallDepth int
}

const minMemory = 4 * 1024 * 1024

func (c *Config) initDefaults() {
	if c.DefaultMemory == 0 {
		c.DefaultMemory = 16 * 1024 * 1024
	}
	c.DefaultMemory = max(c.DefaultMemory, minMemory)
	if c.WorkerMemory == 0 {
		c.WorkerMemory = 128 * 1024 * 1024
	}
	c.WorkerMemory = max(c.WorkerMemory, c.DefaultMemory)
	if c.TotalMemory == 0 {
		c.TotalMemory = 512 * 1024 * 1024
	}
	c.TotalMemory = max(c.TotalMemory, c.WorkerMemory)
	if c.MaxWorkers == 0 {
		c.MaxWorkers = 16
	}
	if c.JobTimeout == 0 {
		c.JobTimeout = 30 * time.Second
	}
	if c.AsyncAfter == 0 {
		c.AsyncAfter = 10 * time.Second
	}
	if c.MaxQueue == 0 {
		c.MaxQueue = 100
	}
	if c.MaxCallDepth == 0 {
		c.MaxCallDepth = 1000
	}
}

type runtime struct {
	config    Config
	tasks     chan task.WaitingTask
	reports   chan int
	initGuard atomic.Bool
}

func (r *runtime) submit(params task.Params, done func()) (<-chan task.Result, context.CancelCauseFunc) {
	r.init()
	params.Memory = min(max(params.Memory, r.config.DefaultMemory), r.config.WorkerMemory)
	if params.Timeout <= 0 || params.Timeout > r.config.JobTimeout {
		params.Timeout = r.config.JobTimeout
	}
	task, res, cancel := task.NewTask(params, done, r.config.AsyncAfter)
	r.tasks <- task
	return res, cancel
}

func (r *runtime) init() {
	if !r.initGuard.CompareAndSwap(false, true) {
		return
	}
	go r.loop()
}

func (r *runtime) loop() {
	queue := deque.Deque[task.WaitingTask]{}
	used := 0
	active := 0
	for {
		for queue.Len() != 0 {
			task := queue.Front()
			select {
			case <-task.Done():
				queue.PopFront()
				continue
			default:
			}
			if active >= r.config.MaxWorkers || used+task.Memory() > r.config.TotalMemory {
				break
			}
			queue.PopFront()
			used += task.Memory()
			active += 1
			go r.run(task)
		}
		var front <-chan struct{}
		if queue.Len() != 0 {
			front = queue.Front().Done()
		}
		select {
		case task := <-r.tasks:
			if queue.Len() >= r.config.MaxQueue {
				slog.Warn("js: queue full, script rejected", "script", task.Name(), "queue", queue.Len())
				task.Reject(errBusy)
				continue
			}
			queue.PushBack(task)
		case memory := <-r.reports:
			used -= memory
			active -= 1
		case <-front:
		}
	}
}

func (r *runtime) run(task task.WaitingTask) {
	defer func() { r.reports <- task.Memory() }()
	vm, err := quickjs.NewVM()
	if err != nil {
		task.Reject(fmt.Errorf("%v: %w", err, errRuntime))
		return
	}
	defer vm.Close()
	executing, ok := task.Execute(vm.Interrupt)
	if !ok {
		return
	}
	if out, err := r.execute(vm, executing.Input()); err == nil {
		executing.Ok(out)
	} else {
		executing.Err(err)
	}
}

func (r *runtime) execute(vm *quickjs.VM, input task.Input) (out string, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			out = ""
			err = fmt.Errorf("%q: %v: %w", input.Name, rec, errRuntime)
		}
	}()
	vm.SetMemoryLimit(uintptr(input.Memory))
	vm.SetMaxStackSize(uintptr(r.config.MaxCallDepth))
	if !input.AppendLogs {
		if err := ClearLog(input.Controller.FS(), logName(input.Name)); err != nil {
			return "", fmt.Errorf("%q: %v: %w", input.Name, err, errRuntime)
		}
	}
	if err := RegisterSerializers(vm); err != nil {
		return "", fmt.Errorf("%q: %v: %w", input.Name, err, errRuntime)
	}
	if err := RegisterLoggers(vm, input.Controller.FS(), logName(input.Name)); err != nil {
		return "", fmt.Errorf("%q: %v: %w", input.Name, err, errRuntime)
	}
	if err := RegisterModules(vm, input.Context, input.Controller.Modules()); err != nil {
		return "", fmt.Errorf("%q: %v: %w", input.Name, err, errRuntime)
	}
	res, err := vm.Eval(input.Script, quickjs.EvalGlobal)
	if err != nil {
		if errors.Is(context.Cause(input.Context), task.ErrTimeout) {
			return "", fmt.Errorf("%q: limit %v: %w", input.Name, input.Timeout, task.ErrTimeout)
		}
		return "", fmt.Errorf("%q: %v: %w", input.Name, err, errScript)
	}
	s, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("%q: the returned value does not serialize to JSON, such as a function, a symbol or a toJSON returning undefined: %w", input.Name, errScript)
	}
	return s, nil
}
