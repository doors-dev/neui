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

package thread

import (
	"context"
	"errors"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/storage/local"
)

func TestRenameUnlinkedThread(t *testing.T) {
	ctx := context.Background()
	s := local.NewMessageStorage()
	created, err := s.ThreadCreate(ctx, "rename-cold")
	if err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	if err := Rename(ctx, s, "rename-cold", "named"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	th, err := s.ThreadGet(ctx, "rename-cold")
	if err != nil {
		t.Fatalf("ThreadGet: %v", err)
	}
	if th.Name != "named" {
		t.Fatalf("name after Rename: got %q, want %q", th.Name, "named")
	}
	if th.Seq != created.Seq+1 {
		t.Fatalf("seq after Rename: got %d, want %d", th.Seq, created.Seq+1)
	}
	if _, ok := registry.Load(api.ThreadID("rename-cold")); !ok {
		t.Fatalf("Rename left no thread in the registry")
	}
	if err := Rename(ctx, s, "rename-cold", ""); err != nil {
		t.Fatalf("Rename back to unnamed: %v", err)
	}
	th, err = s.ThreadGet(ctx, "rename-cold")
	if err != nil {
		t.Fatalf("ThreadGet: %v", err)
	}
	if th.Name != "" || th.Seq != created.Seq+2 {
		t.Fatalf("thread after the second Rename: %+v", th)
	}
	if err := Rename(ctx, s, "rename-ghost", "x"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("Rename of a thread that is not in storage: want ErrNotFound, got %v", err)
	}
}

func TestDeleteUnlinkedThread(t *testing.T) {
	ctx := context.Background()
	s := local.NewMessageStorage()
	if _, err := s.ThreadCreate(ctx, "delete-cold"); err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	// the thread runs before the command arrives, so the delete routes through it
	getThread("delete-cold")
	if err := Delete(ctx, s, "delete-cold"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.ThreadGet(ctx, "delete-cold"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ThreadGet after Delete: want ErrNotFound, got %v", err)
	}
	if err := Delete(ctx, s, "delete-cold"); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
	if err := Delete(ctx, s, "delete-ghost"); err != nil {
		t.Fatalf("Delete of a thread that was never there: %v", err)
	}
}
