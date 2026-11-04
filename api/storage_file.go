package api

import (
	"context"
	"io"
	"iter"
	"time"
)

// Origin says which side of the conversation a file came from. It is part of
// the key, so the two sides are separate namespaces.
type Origin string

const (
	OriginUser  Origin = "user"  // uploaded by the user
	OriginAgent Origin = "agent" // stored by the model or its tools
)

// Valid reports whether o is an origin storage knows; FilePut refuses anything
// else.
func (o Origin) Valid() bool {
	switch o {
	case OriginUser, OriginAgent:
		return true
	default:
		return false
	}
}

// File is the stored record: what the caller supplied plus what storage
// measured on the way in.
type File struct {
	ThreadID  ThreadID  `json:"thread_id"`  // thread the file belongs to
	Name      string    `json:"name"`       // key within the thread and origin
	MIME      string    `json:"mime"`       // as the caller supplied it to FilePut
	Origin    Origin    `json:"origin"`     // which side stored it
	Size      int64     `json:"size"`       // bytes, measured by storage
	CreatedAt time.Time `json:"created_at"` // when FilePut committed it
	// Draft marks a blob stored ahead of the message that will carry it. A
	// draft is not part of the conversation: FileStorage lists it like any
	// file, but FS leaves it out of Stat and ReadDir. It stays a draft until
	// FileCommit clears it or it is deleted.
	Draft bool `json:"draft"`
}

// FileStorage keeps immutable blobs under caller-chosen (thread, origin,
// name) keys. The contract mirrors S3 throughout, so a remote implementation
// can stand in for a local one unchanged. The blob never changes; the one
// mutable bit of the record is Draft, and FileCommit is the only way to flip
// it.
type FileStorage interface {
	// FilePut streams r into a new blob and returns the record with Size
	// and CreatedAt filled. Name is the key within the thread and origin:
	// one path element, not empty, not "." or ".." and holding no
	// separator — anything else is ErrInvalidName, so every stored name is
	// addressable. An empty threadID or an origin that is not Valid is
	// refused too. It commits whole or not at all, and a name already
	// taken is ErrExists with nothing written: blobs are immutable, so the
	// caller resolves a collision by choosing another name (on S3, a
	// conditional If-None-Match put). draft stores the blob as a draft, see
	// File.Draft.
	FilePut(ctx context.Context, threadID ThreadID, origin Origin, name string, mime string, r io.Reader, draft bool) (File, error)
	// FileCommit clears Draft on the record, making the blob part of the
	// conversation; a committed file stays committed. A missing key is
	// ErrNotFound. On a keyed store this is a metadata-only change (a tag,
	// a row update), never a copy.
	FileCommit(ctx context.Context, threadID ThreadID, origin Origin, name string) error
	// FileDiscardDrafts deletes the origin's drafts except those named in
	// keep, so a caller sweeps abandoned uploads in one call. A missing
	// thread or origin is nil.
	FileDiscardDrafts(ctx context.Context, threadID ThreadID, origin Origin, keep []string) error
	// FileStat returns the record; a missing key is ErrNotFound.
	FileStat(ctx context.Context, threadID ThreadID, origin Origin, name string) (File, error)
	// FileRead streams [off, off+n) of the blob; n < 0 means to EOF.
	// Reading past EOF truncates silently; a negative off is refused and
	// off at or past the end is ErrOutOfRange, except off 0 with negative
	// n on an empty blob, which reads empty (an S3 ranged GET answers 416
	// the same way). A missing key is ErrNotFound. The reader does not
	// watch ctx; closing it closes the blob.
	FileRead(ctx context.Context, threadID ThreadID, origin Origin, name string, off int64, n int64) (io.ReadCloser, error)
	// FileList yields the origin's files sorted lexically by name in byte
	// order, which is what a keyed store lists in. A missing thread or
	// origin yields nothing rather than an error. A record that cannot be
	// read comes out as a zero File with its error and iteration continues,
	// so one bad entry never hides the rest.
	FileList(ctx context.Context, threadID ThreadID, origin Origin) iter.Seq2[File, error]
	// FileDelete removes the blob and its record; a missing key is nil, so
	// deletes are idempotent.
	FileDelete(ctx context.Context, threadID ThreadID, origin Origin, name string) error
	// FileDeleteAll removes every blob in the thread, all origins; a
	// missing thread is nil, so it too is idempotent.
	FileDeleteAll(ctx context.Context, threadID ThreadID) error
}
