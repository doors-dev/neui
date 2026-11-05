package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/filesystem"
	"neui.dev/internal/storage/local"
)

type testThread struct {
	fs api.FS
}

func (testThread) ID() api.ThreadID { return "thread" }

func (testThread) Context() context.Context { return context.Background() }

func (testThread) PageContext() context.Context { return context.Background() }

func (testThread) Modules() iter.Seq[common.Module] {
	return func(func(common.Module) bool) {}
}

func (t testThread) FS() api.FS               { return t.fs }
func (t testThread) Model() api.Model         { return api.Model{} }
func (t testThread) Config() api.ThreadConfig { return api.ThreadConfig{} }

func (testThread) FilesConfig() api.FilesConfig {
	return common.FilesConfigDefaults(api.FilesConfig{})
}

func (testThread) Task(string, map[string]any, context.CancelCauseFunc) (common.Task, error) {
	return nil, errors.New("no tasks in tests")
}

func (testThread) Ask(api.ChatAsk) (func(context.Context), error) {
	return nil, errors.New("no asks in tests")
}

func (testThread) Rename(context.Context, string) error {
	return errors.New("no renames in tests")
}

func TestFetchSaveAsStreamsToWorkspace(t *testing.T) {
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Remove() })
	ws, err := store.WorkspaceOpen("thread")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	fs := filesystem.New("thread", ws, store, api.FilesConfig{})
	ctx := context.WithValue(context.Background(), common.KeyController, common.Controller(testThread{fs: fs}))
	body := "%PDF-1.7\x00 binary-ish payload"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	m := Module(Config{HTTPClient: srv.Client()})(ctx)
	tool, ok := m.Get("fetch")
	if !ok {
		t.Fatal("no fetch tool")
	}
	out, err := tool.Agent(ctx, json.RawMessage(fmt.Sprintf(`{"url":%q,"saveAs":"papers/spec.pdf"}`, srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	text := out[0].(api.ContentText).Text
	t.Log(text)
	if !strings.Contains(text, `savedAs="papers/spec.pdf"`) || !strings.Contains(text, fmt.Sprintf(`bytes="%d"`, len(body))) {
		t.Errorf("saveAs attributes wrong: %s", text)
	}
	if !strings.Contains(text, `contentType="application/pdf"`) || strings.Contains(text, "</fetched>") {
		t.Errorf("saveAs must render an empty element: %s", text)
	}
	rc, err := fs.ReadAll(ctx, "papers/spec.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	written, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != body {
		t.Errorf("workspace holds %q", written)
	}
}

func TestFetchSaveAsOverTheCapKeepsNothing(t *testing.T) {
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Remove() })
	ws, err := store.WorkspaceOpen("thread")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	fs := filesystem.New("thread", ws, store, api.FilesConfig{})
	ctx := context.WithValue(context.Background(), common.KeyController, common.Controller(testThread{fs: fs}))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		if r.URL.Path == "/sized" {
			w.Header().Set("Content-Length", "4096")
		}
		w.Write([]byte(strings.Repeat("x", 4096)))
	}))
	defer srv.Close()
	m := Module(Config{HTTPClient: srv.Client(), MaxBytes: 1024})(ctx)
	tool, ok := m.Get("fetch")
	if !ok {
		t.Fatal("no fetch tool")
	}
	for _, path := range []string{"/chunked", "/sized"} {
		_, err := tool.Agent(ctx, json.RawMessage(fmt.Sprintf(`{"url":%q,"saveAs":"big.pdf"}`, srv.URL+path)))
		if err == nil {
			t.Fatalf("%s: a body over the cap must be an error", path)
		}
		if !strings.Contains(err.Error(), "is over the 1024-byte cap and was not saved") {
			t.Errorf("%s: the saveAs path must not advise passing saveAs: %v", path, err)
		}
		if _, serr := fs.Stat(ctx, "big.pdf"); serr == nil {
			t.Errorf("%s: a truncated file was left in the workspace", path)
		}
	}
}

func TestFetchSaveAsFailureKeepsExistingFile(t *testing.T) {
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Remove() })
	ws, err := store.WorkspaceOpen("thread")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	fs := filesystem.New("thread", ws, store, api.FilesConfig{})
	ctx := context.WithValue(context.Background(), common.KeyController, common.Controller(testThread{fs: fs}))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		switch r.URL.Path {
		case "/big":
			w.Write([]byte(strings.Repeat("x", 4096)))
		case "/reset":
			w.Header().Set("Content-Length", "512")
			w.Write([]byte(strings.Repeat("y", 100)))
			w.(http.Flusher).Flush()
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
		default:
			w.Write([]byte("fresh"))
		}
	}))
	defer srv.Close()
	read := func() string {
		rc, err := fs.ReadAll(ctx, "docs/spec.pdf")
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		b, _ := io.ReadAll(rc)
		return string(b)
	}
	put, err := fs.Create("docs/spec.pdf")
	if err != nil {
		t.Fatal(err)
	}
	put.Write([]byte("good copy"))
	put.Close()
	m := Module(Config{HTTPClient: srv.Client(), MaxBytes: 1024})(ctx)
	tool, _ := m.Get("fetch")
	for _, p := range []string{"/big", "/reset"} {
		if _, err := tool.Agent(ctx, json.RawMessage(fmt.Sprintf(`{"url":%q,"saveAs":"docs/spec.pdf"}`, srv.URL+p))); err == nil {
			t.Fatalf("%s: want an error", p)
		}
		if got := read(); got != "good copy" {
			t.Errorf("%s: a failed fetch replaced the existing file with %q", p, got)
		}
	}
	entries, err := fs.ReadDir(ctx, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("partial downloads were left behind: %+v", entries)
	}
	if _, err := tool.Agent(ctx, json.RawMessage(fmt.Sprintf(`{"url":%q,"saveAs":"docs/spec.pdf"}`, srv.URL+"/ok"))); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "fresh" {
		t.Errorf("a successful fetch must replace the file, got %q", got)
	}
}
