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

package common

import (
	"context"
	"testing"

	"neui.dev/api"
)

func TestFilesConfigDefaults(t *testing.T) {
	c := FilesConfigDefaults(api.FilesConfig{MaxBytes: -1, ReadLimit: 64})
	if c.MaxBytes != -1 {
		t.Fatalf("MaxBytes = %d, want the configured -1", c.MaxBytes)
	}
	if c.ReadLimit != 64 {
		t.Fatalf("ReadLimit = %d, want the configured 64", c.ReadLimit)
	}
	if c.MaxFiles != 10000 {
		t.Fatalf("MaxFiles = %d, want 10000", c.MaxFiles)
	}
	if c.EditLimit != 10<<20 {
		t.Fatalf("EditLimit = %d, want %d", c.EditLimit, 10<<20)
	}
	if c.ImageFileLimit != 20<<20 {
		t.Fatalf("ImageFileLimit = %d, want %d", c.ImageFileLimit, 20<<20)
	}
	if c.DocumentLimit != 32<<20 {
		t.Fatalf("DocumentLimit = %d, want %d", c.DocumentLimit, 32<<20)
	}
	if c.UnzipLimit != 512<<20 {
		t.Fatalf("UnzipLimit = %d, want %d", c.UnzipLimit, 512<<20)
	}
	if c.UnzipFiles != 10000 {
		t.Fatalf("UnzipFiles = %d, want 10000", c.UnzipFiles)
	}
	if FilesConfigDefaults(c) != c {
		t.Fatal("resolving twice changed the result")
	}
}

func TestAppFilesConfigDefaults(t *testing.T) {
	c := AppFilesConfig(context.Background())
	if c.ReadLimit != 50<<10 {
		t.Fatalf("ReadLimit = %d, want %d", c.ReadLimit, 50<<10)
	}
	if c.MaxBytes != 1<<30 {
		t.Fatalf("MaxBytes = %d, want %d", c.MaxBytes, 1<<30)
	}
}

func TestReplayItemShareRange(t *testing.T) {
	for share, want := range map[float32]float32{-1: 0.1, 0: 0.1, 0.2: 0.2, 1: 0.25} {
		if got := ConfigDefaults(api.ThreadConfig{ReplayItemShare: share}).ReplayItemShare; got != want {
			t.Fatalf("ReplayItemShare %v resolves to %v, want %v", share, got, want)
		}
	}
}
