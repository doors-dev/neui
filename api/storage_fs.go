package api

import (
	"context"
	"io"
	"time"
)

// UserUploadsDir and AgentUploadsDir are the virtual directories under which FS
// serves the thread's stored files (OriginUser and OriginAgent); every other
// path is the workspace on disk.
const (
	UserUploadsDir  = "user_uploads"
	AgentUploadsDir = "agent_uploads"
)

// FSEntry is one name in the merged filesystem view.
type FSEntry struct {
	Name    string    // workspace-relative path
	Size    int64     // bytes; for directories whatever the OS reports (0 for the virtual dirs)
	Dir     bool      // a directory rather than a file
	ModTime time.Time // last write for local files, upload time for virtual ones
	MIME    string    // File.MIME for virtual entries, extension-derived for local files; "" when unknown or a directory
	Virtual bool      // served from file storage, not workspace disk: reads download, LocalPath refuses
}

// FS is the filesystem a thread's tools see: the workspace merged with the
// thread's stored files, which appear under the virtual user_uploads/ and
// agent_uploads/ dirs and are streamed straight from file storage on read;
// nothing materializes locally. user_uploads/ is read-only; agent_uploads/
// takes new files only through Copy and loses them only through Remove or
// DeleteUpload. Stat and ReadDir leave out drafts; UploadStat reports them.
type FS interface {
	// ReadRange streams [off, off+n) of the named file; n < 0 means to EOF.
	// Reading past EOF truncates silently; a negative off is refused and
	// off >= size returns ErrOutOfRange, except off 0 with negative n on
	// an empty file, which reads empty.
	ReadRange(ctx context.Context, name string, off, n int64) (io.ReadCloser, error)
	// ReadAll streams the whole file, wherever it lives.
	ReadAll(ctx context.Context, name string) (io.ReadCloser, error)
	// LocalPath returns the absolute local path of a workspace file, for
	// libraries that need io.ReaderAt or a real path. A virtual path is
	// ErrVirtual (copy it into the workspace first), and a missing one is
	// ErrNotFound.
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
	// agent_uploads/ file from file storage, freeing its name. user_uploads/
	// and the agent_uploads/ dir itself return ErrReadOnly, and a TempDir path
	// ErrReserved. A missing name is not an error.
	Remove(ctx context.Context, name string) error
	// Stat reports a name's entry from the merged view. Virtual entries carry
	// File.MIME, local files an extension-derived one; "" when unknown. A
	// missing name or a draft upload is ErrNotFound.
	Stat(ctx context.Context, name string) (FSEntry, error)
	// ReadDir lists a directory of the merged view, "." for root, which also
	// lists both uploads dirs. Uploads entries show final (deduplicated) names
	// and leave out drafts.
	ReadDir(ctx context.Context, name string) ([]FSEntry, error)
	// UserUpload streams r into file storage as a user_uploads/ draft, named by
	// the base name of the client-provided name ("unnamed" when that is empty,
	// "/", "." or ".."), with a "(N)" suffix before the extension when the name
	// is taken, and its MIME sniffed from the first 512 bytes. It returns the
	// draft's entry; the draft stays hidden from Stat and ReadDir until it is
	// committed with FileStorage.FileCommit.
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
