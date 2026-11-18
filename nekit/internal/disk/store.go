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
