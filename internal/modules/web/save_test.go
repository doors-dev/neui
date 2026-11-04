package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/testkit"
)

func TestFetchSaveAsStreamsToWorkspace(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs, ctx := c.FS(), c.Ctx()
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
	c := testkit.NewController(t, testkit.Setup{})
	fs, ctx := c.FS(), c.Ctx()
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
