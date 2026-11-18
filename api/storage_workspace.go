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

package api

import "os"

// Workspace is a thread's scratch directory. File access goes through
// Root, which cannot be escaped by .. or symlinks; Dir is the absolute path,
// for what needs a real path, such as a cwd or a library that opens files
// itself.
type Workspace struct {
	Dir  string   // absolute path; use Root for file access
	Root *os.Root // confined handle every file access goes through
}

// Close releases the handle and nothing else: the directory stays, so the
// thread finds its files on the next open, and a second handle on the same
// workspace keeps working.
func (w *Workspace) Close() error {
	return w.Root.Close()
}

// WorkspaceStorage hands out per-thread scratch directories. A workspace is
// created on first open and survives Close, so a thread finds its files again
// until the workspace is deleted.
type WorkspaceStorage interface {
	// WorkspaceOpen returns the thread's workspace, creating it on first use.
	WorkspaceOpen(threadID ThreadID) (Workspace, error)
	// WorkspaceDelete removes the workspace with everything in it; a missing
	// one is nil, so deletes are idempotent. A handle still open keeps its
	// descriptor but the files are gone.
	WorkspaceDelete(threadID ThreadID) error
}
