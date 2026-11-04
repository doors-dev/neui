package api

import (
	"context"
	"io"
	"time"
)

// UserUploadsDir and AgentUploadsDir are the virtual dirs the FS serves
// stored files under; everything else in the workspace is transient.
const (
	UserUploadsDir  = "user_uploads"
	AgentUploadsDir = "agent_uploads"
)

// FSEntry is one name in the merged filesystem view.
type FSEntry struct {
	Name    string    // workspace-relative path
	Size    int64     // bytes; 0 for directories
	Dir     bool      // a directory rather than a file
	ModTime time.Time // last write for local files, upload time for virtual ones
	MIME    string    // manifest MIME for virtual entries, extension-derived for local files; "" when unknown
	Virtual bool      // served from file storage, not workspace disk: reads download, LocalPath refuses
}

// FS is the filesystem a thread's tools see: the workspace merged with the
// thread's stored files, the latter read-only under the virtual
// user_uploads/ and agent_uploads/ dirs and streamed straight from file
// storage on read; nothing materializes locally.
type FS interface {
	// ReadRange streams [off, off+n) of the named file; n < 0 means to EOF.
	// Reading past EOF truncates silently; a negative off is refused and
	// off >= size returns ErrOutOfRange, except off 0 with negative n on
	// an empty file, which reads empty.
	ReadRange(ctx context.Context, name string, off, n int64) (io.ReadCloser, error)
	// ReadAll streams the whole file, wherever it lives.
	ReadAll(ctx context.Context, name string) (io.ReadCloser, error)
	// LocalPath returns the absolute local path of a workspace file, for
	// libraries that need io.ReaderAt or a real path.
	// Virtual entries have no local path and are refused; copy into the
	// workspace first.
	LocalPath(ctx context.Context, name string) (string, error)
	// TempDir returns the absolute local path of a scratch directory
	// reserved for libraries that do their own filesystem work and cannot
	// be routed through this interface, such as unpacking an archive,
	// creating it on first call. It is not workspace content: it stays out
	// of ReadDir and out of the FilesConfig workspace limits, every other
	// method refuses paths inside it with ErrReserved, and it is wiped once
	// a round's tool calls are done, so work outliving the round copies
	// what it needs into the workspace first.
	TempDir() (string, error)
	// Create opens a local file for writing, truncating if it exists,
	// creating parent directories as needed. Returns ErrReadOnly under
	// the uploads dirs, and ErrQuota when the FilesConfig workspace limits
	// are reached — at open or mid-write; Append and local Copy share both
	// refusals.
	Create(name string) (io.WriteCloser, error)
	// Append is Create without truncation, positioned at EOF.
	Append(name string) (io.WriteCloser, error)
	// Copy copies src to dst host-side (no caller byte shuttling).
	// src may be virtual; dst must be local-writable or
	// agent_uploads/<name>, which uploads to file storage instead of
	// writing locally — a name already taken is ErrExists. This is the
	// blessed "edit an upload" path.
	Copy(ctx context.Context, src, dst string) error
	// Rename moves a local file or directory. Either side under an
	// uploads dir returns ErrReadOnly.
	Rename(oldname, newname string) error
	// Remove deletes a local file or directory recursively, or an
	// agent_uploads/ file from file storage, freeing its name — the agent
	// owns that namespace. user_uploads/ returns ErrReadOnly. Missing dst
	// is not an error (ensure-absent), but a TempDir path is refused like
	// anywhere else: it exists and is not the caller's to delete.
	Remove(ctx context.Context, name string) error
	// Stat reports metadata from the merged view (manifest ∪ local).
	// Virtual entries carry manifest MIME, local files an
	// extension-derived one; "" when unknown.
	Stat(ctx context.Context, name string) (FSEntry, error)
	// ReadDir lists a directory of the merged view, "." for root.
	// Uploads entries show final (deduplicated) names.
	ReadDir(ctx context.Context, name string) ([]FSEntry, error)
	// UserUpload streams r into file storage under the client-provided
	// name, reduced to a safe base name and MIME sniffed from the content,
	// and returns the entry as user_uploads/ lists it, dedup suffix
	// included.
	UserUpload(ctx context.Context, name string, r io.Reader) (FSEntry, error)
	// UploadStat returns an upload's entry whether or not it is still a
	// draft; Stat hides drafts, this is for whoever manages them. The name
	// is the user_uploads/ or agent_uploads/ path as listed.
	UploadStat(ctx context.Context, name string) (FSEntry, error)
	// DeleteUpload removes an upload from file storage; the name is the
	// user_uploads/ or agent_uploads/ path as listed. Missing is not an
	// error (ensure-absent).
	DeleteUpload(ctx context.Context, name string) error
}
