package api

import "os"

// Workspace is a thread's scratch directory. Every path a model or a tool
// produces goes through Root, which cannot be escaped by .. or symlinks; Dir
// exists to hand a cwd to an exec'd process.
type Workspace struct {
	Dir  string
	Root *os.Root
}

// Close releases the handle and nothing else: the directory stays, so the
// thread finds its files on the next open, and a second handle on the same
// workspace keeps working.
func (w *Workspace) Close() error {
	return w.Root.Close()
}

// WorkspaceStorage hands out per-thread scratch directories. A workspace is
// created on first open and survives Close, so a thread finds its files again;
// only WorkspaceDelete takes them away.
type WorkspaceStorage interface {
	// WorkspaceOpen returns the thread's workspace, creating it on first use.
	WorkspaceOpen(threadID ThreadID) (Workspace, error)
	// WorkspaceDelete removes the workspace with everything in it; a missing
	// one is nil, so deletes are idempotent. A handle still open keeps its
	// descriptor but the files are gone.
	WorkspaceDelete(threadID ThreadID) error
}
