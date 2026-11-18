package disk

import (
	"context"
	"io"
	"iter"

	"neui.dev/api"
	"neui.dev/internal/storage/local"
)

// Store keeps uploaded files in a directory on the local disk, where they
// survive restarts. Plug it in with neui.WithUploadStorage or
// neui.OptionUploadStorage.
type Store struct {
	files *local.FileStorage
}

func New(path string) (*Store, error) {
	files, err := local.OpenFileStorage(path)
	if err != nil {
		return nil, err
	}
	return &Store{files: files}, nil
}

func (s *Store) FilePut(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string, mime string, r io.Reader, draft bool) (api.File, error) {
	return s.files.FilePut(ctx, threadID, origin, name, mime, r, draft)
}

func (s *Store) FileCommit(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string) error {
	return s.files.FileCommit(ctx, threadID, origin, name)
}

func (s *Store) FileDiscardDrafts(ctx context.Context, threadID api.ThreadID, origin api.Origin, keep []string) error {
	return s.files.FileDiscardDrafts(ctx, threadID, origin, keep)
}

func (s *Store) FileStat(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string) (api.File, error) {
	return s.files.FileStat(ctx, threadID, origin, name)
}

func (s *Store) FileRead(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string, off int64, n int64) (io.ReadCloser, error) {
	return s.files.FileRead(ctx, threadID, origin, name, off, n)
}

func (s *Store) FileList(ctx context.Context, threadID api.ThreadID, origin api.Origin) iter.Seq2[api.File, error] {
	return s.files.FileList(ctx, threadID, origin)
}

func (s *Store) FileDelete(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string) error {
	return s.files.FileDelete(ctx, threadID, origin, name)
}

func (s *Store) FileDeleteAll(ctx context.Context, threadID api.ThreadID) error {
	return s.files.FileDeleteAll(ctx, threadID)
}
