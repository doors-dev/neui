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

package marksman

import (
	"context"
	"io"
	"net/http/cookiejar"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
)

type svgFS struct {
	api.FS
}

func (svgFS) Stat(ctx context.Context, name string) (api.FSEntry, error) {
	return api.FSEntry{Name: name, MIME: "image/svg+xml", Size: 4}, nil
}

func (svgFS) ReadAll(ctx context.Context, name string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("<svg")), nil
}

func TestServeFileSandboxed(t *testing.T) {
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Elem {
		return gox.Elem(func(cur gox.Cursor) error {
			return renderFileImage(cur, svgFS{}, api.AgentUploadsDir+"/x.svg", "")
		})
	})
	srv := httptest.NewServer(app)
	defer srv.Close()
	client := srv.Client()
	client.Jar, _ = cookiejar.New(nil)
	res, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	m := regexp.MustCompile(`src="([^"]+)"`).FindSubmatch(body)
	if m == nil {
		t.Fatalf("no img src in %q", body)
	}
	img, err := client.Get(srv.URL + strings.ReplaceAll(string(m[1]), "&amp;", "&"))
	if err != nil {
		t.Fatalf("get image: %v", err)
	}
	img.Body.Close()
	if img.StatusCode != 200 {
		t.Fatalf("image status %d", img.StatusCode)
	}
	if got := img.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := img.Header.Get("Content-Security-Policy"); got != "sandbox" {
		t.Fatalf("Content-Security-Policy = %q, want sandbox", got)
	}
}
