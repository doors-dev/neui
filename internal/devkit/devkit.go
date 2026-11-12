// Package devkit runs an island app from a script, with no agent in the
// loop, for development: Go tests drive it with a headless browser, and a
// program under cmd/ serves it to try widgets by hand.
//
// The Kit is an http.Handler serving one page with the island. Every page
// open renders a fresh island and, when App.Script is set, starts the script
// on it; Start, Continue and Stop act on the page opened last. What the
// script reports with ui.notify, and how each run ends, goes to App.Report.
//
//	func main() {
//		kit, err := devkit.New(devkit.App{
//			Island: neui.Island{
//				Name:     "board",
//				Template: func(slot gox.Comp) gox.Elem { return func(cur gox.Cursor) error { return cur.Comp(slot) } },
//				Library: func(l neui.Library) error {
//					l.Head(ui.Style())
//					return l.Control("form", "a form", ui.NewForm)
//				},
//			},
//			Script: `ui.root.set(ui.form({fields: [{type: "text", label: "Name", name: "name"}]})); while (true) ui.notify(ui.draw())`,
//		})
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer kit.Close()
//		log.Fatal(http.ListenAndServe(":8080", kit))
//	}
package devkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/filesystem"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/utils"
)

// App is the island the Kit serves and how it runs.
type App struct {
	// Island is rendered as the page's only content; an empty Name becomes
	// "island".
	Island neui.Island
	// Config limits the runs; zero fields take the defaults.
	Config api.IslandConfig
	// Files limits the workspace, such as the size of a file a form
	// uploads; zero fields take the defaults.
	Files api.FilesConfig
	// Dir is the workspace directory; empty uses a temporary one that Close
	// removes.
	Dir string
	// Script starts on every page open; empty starts nothing.
	Script string
	// Report receives what the runs report; nil prints it to stderr.
	Report func(Report)
}

// Report is a notification or the end of a run.
type Report struct {
	// Task is the run's id; 0 when a script failed to start.
	Task uint
	// Kind is "notify", "ok" or "error".
	Kind string
	// Data is the notification or the result.
	Data json.RawMessage
	// Meta is the notification's meta, such as its kind and script.
	Meta map[string]any
	// Err is the error the run ended or failed to start with.
	Err error
}

func (r Report) String() string {
	switch r.Kind {
	case "error":
		return fmt.Sprintf("task %d error: %v", r.Task, r.Err)
	case "ok":
		return fmt.Sprintf("task %d ok: %s", r.Task, r.Data)
	default:
		return fmt.Sprintf("task %d %v %v: %s", r.Task, r.Meta["kind"], r.Meta["script"], r.Data)
	}
}

// Kit serves the island page and runs scripts on it.
type Kit struct {
	app     App
	handler http.Handler
	store   *local.FileStorage
	fs      filesystem.FS
	mu      sync.Mutex
	tasks   uint
	current *controller
}

// New creates the Kit; Close releases its workspace.
func New(a App) (*Kit, error) {
	store, err := local.NewFileStorage("")
	if err != nil {
		return nil, err
	}
	var storage api.WorkspaceStorage = store
	if a.Dir != "" {
		storage = workspace(a.Dir)
	}
	if a.Island.Name == "" {
		a.Island.Name = "island"
	}
	if a.Report == nil {
		a.Report = func(r Report) { fmt.Fprintln(os.Stderr, r) }
	}
	k := &Kit{
		app:   a,
		store: store,
		fs:    filesystem.Open(threadID, storage, store, common.FilesConfigDefaults(a.Files)),
	}
	k.handler = doors.NewApp(func(context.Context, doors.Request) gox.Elem {
		return k.page()
	}, doors.WithID(doors.IDRand()))
	return k, nil
}

func (k *Kit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	k.handler.ServeHTTP(w, r)
}

// Start starts code on the island of the page opened last, ending the run
// there, and returns the run's task id and the script number.
func (k *Kit) Start(ctx context.Context, code string) (uint, uint, error) {
	c, err := k.controller()
	if err != nil {
		return 0, 0, err
	}
	return c.start(ctx, code)
}

// Continue queues code on the running app of the page opened last;
// interrupt drops the queue and stops the script that runs.
func (k *Kit) Continue(code string, interrupt bool) (uint, uint, error) {
	c, err := k.controller()
	if err != nil {
		return 0, 0, err
	}
	island, _, err := c.page.Island(c.page.Context(), c.path)
	if err != nil {
		return 0, 0, err
	}
	return island.ContinueCode(code, interrupt)
}

// Stop ends the running app of the page opened last and waits for it.
func (k *Kit) Stop(ctx context.Context) error {
	c, err := k.controller()
	if err != nil {
		return err
	}
	island, _, err := c.page.Island(c.page.Context(), c.path)
	if err != nil {
		return err
	}
	return island.Done(ctx)
}

// Close releases the workspace, removing it unless App.Dir was set.
func (k *Kit) Close() error {
	return errors.Join(k.fs.Dispose(), k.store.Remove())
}

func (k *Kit) controller() (*controller, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.current == nil {
		return nil, errors.New("devkit: no page is open")
	}
	return k.current, nil
}

func (k *Kit) path() string {
	return utils.SanitizeName(k.app.Island.Name)
}
