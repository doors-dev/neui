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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/storage/local"
)

func TestEnableRetriesLocalFileStorage(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	s := &session{ctx: context.Background(), instances: make(map[*instance]struct{})}
	if err := s.Enable(nil); err == nil {
		t.Fatal("Enable succeeded without a temp dir")
	}
	if s.backend != nil || s.localFileStorage != nil {
		t.Fatal("a failed Enable kept a backend or a file storage")
	}
	t.Setenv("TMPDIR", t.TempDir())
	s.Enable(nil)
	if s.localFileStorage == nil {
		t.Fatal("a later Enable did not create the file storage")
	}
}

func TestSessionEndRemovesTheLocalFileStorage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{ctx: ctx, instances: make(map[*instance]struct{})}
	s.Enable(nil)
	if entries, _ := os.ReadDir(dir); s.localFileStorage == nil || len(entries) == 0 {
		t.Fatal("Enable did not create the file storage")
	}
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(entries) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the session ended but its file storage is still there: %v", entries)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAppUploadStorageIsUsedAndSessionOptionOverridesIt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	app, err := local.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), common.UploadStorageKey{}, api.UploadStorage(app)))
	s := &session{ctx: ctx, instances: make(map[*instance]struct{})}
	if err := s.Enable([]common.Option{common.OptionProvider(stubProvider{})}); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if s.backend.UploadStorage() != app {
		t.Fatal("the session does not use the app file storage")
	}
	if s.backend.WorkspaceStorage() != s.localFileStorage {
		t.Fatal("the workspaces left the session's temporary directory")
	}
	own, err := local.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	if err := s.Enable([]common.Option{common.OptionForce(), common.OptionProvider(stubProvider{}), common.OptionUploadStorage(own)}); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if s.backend.UploadStorage() != own {
		t.Fatal("OptionUploadStorage did not override the app file storage")
	}
	if _, err := app.FilePut(ctx, "app", api.OriginUser, "a.txt", "text/plain", strings.NewReader("a"), false); err != nil {
		t.Fatalf("FilePut: %v", err)
	}
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for entries, _ := os.ReadDir(dir); len(entries) != 0; entries, _ = os.ReadDir(dir) {
		if time.Now().After(deadline) {
			t.Fatalf("the session ended but its temporary directory is still there: %v", entries)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := app.FileStat(context.Background(), "app", api.OriginUser, "a.txt"); err != nil {
		t.Fatalf("the session's end touched the app file storage: %v", err)
	}
}
