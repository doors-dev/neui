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
	"neui.dev/internal/tools"
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

type Config struct {
	TotalMemory   int
	WorkerMemory  int
	DefaultMemory int
	MaxWorkers    int
	JobTimeout    time.Duration
	AsyncAfter    time.Duration
	MaxQueue      int
	MaxCallDepth  int
}

const minMemory = 4 * 1024 * 1024

func (c *Config) initDefaults() {
	if c.DefaultMemory <= 0 {
		c.DefaultMemory = 16 * 1024 * 1024
	}
	c.DefaultMemory = max(c.DefaultMemory, minMemory)
	if c.WorkerMemory <= 0 {
		c.WorkerMemory = 128 * 1024 * 1024
	}
	c.WorkerMemory = max(c.WorkerMemory, c.DefaultMemory)
	if c.TotalMemory <= 0 {
		c.TotalMemory = 512 * 1024 * 1024
	}
	c.TotalMemory = max(c.TotalMemory, c.WorkerMemory)
	if c.MaxWorkers <= 0 {
		c.MaxWorkers = 16
	}
	if c.JobTimeout <= 0 {
		c.JobTimeout = 30 * time.Second
	}
	if c.AsyncAfter <= 0 {
		c.AsyncAfter = 10 * time.Second
	}
	if c.MaxQueue <= 0 {
		c.MaxQueue = 100
	}
	if c.MaxCallDepth <= 0 {
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
		if err != nil && errors.Is(context.Cause(input.Context), task.ErrTimeout) {
			err = fmt.Errorf("%q: limit %v: %w", input.Name, input.Timeout, task.ErrTimeout)
		}
	}()
	vm.SetMemoryLimit(uintptr(input.Memory))
	vm.SetMaxStackSize(uintptr(r.config.MaxCallDepth))
	if !input.AppendLogs {
		if err := ClearLog(input.Controller.FS(), logName(input.Name)); err != nil {
			return "", fmt.Errorf("%q: clearing log %q: %w", input.Name, logName(input.Name), err)
		}
	}
	if err := RegisterSerializers(vm); err != nil {
		return "", fmt.Errorf("%q: %v: %w", input.Name, err, errRuntime)
	}
	if err := RegisterLoggers(vm, input.Controller.FS(), logName(input.Name)); err != nil {
		return "", fmt.Errorf("%q: %v: %w", input.Name, err, errRuntime)
	}
	if err := RegisterModules(vm, tools.WithRun(input.Context), input.Controller.Modules()); err != nil {
		return "", fmt.Errorf("%q: %v: %w", input.Name, err, errRuntime)
	}
	res, err := vm.Eval(input.Script, quickjs.EvalGlobal)
	if err != nil {
		return "", fmt.Errorf("%q (log file %q): %v: %w", input.Name, logName(input.Name), err, errScript)
	}
	s, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("%q (log file %q): the returned value does not serialize to JSON, such as a function, a symbol or a toJSON returning undefined: %w", input.Name, logName(input.Name), errScript)
	}
	return s, nil
}
