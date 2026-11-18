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

package filesystem

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"

	"neui.dev/api"
)

type quota struct {
	config  api.FilesConfig
	mu      sync.Mutex
	scanned bool
	files   int
	bytes   int64
}

func (q *quota) active() bool {
	return q != nil && (q.config.MaxFiles > 0 || q.config.MaxBytes > 0)
}

func (q *quota) ensure(root fs.FS, temp string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.scanned {
		return nil
	}
	files, bytes, err := usage(root, ".", temp)
	if err != nil {
		return err
	}
	q.files, q.bytes, q.scanned = files, bytes, true
	return nil
}

func usage(root fs.FS, dir string, temp string) (int, int64, error) {
	files, bytes := 0, int64(0)
	err := fs.WalkDir(root, dir, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			if isUploadsDirName(path) || strings.EqualFold(path, temp) {
				return fs.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		files += 1
		bytes += info.Size()
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return files, bytes, nil
}

func (q *quota) admit(newFile bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if newFile && q.config.MaxFiles > 0 && q.files+1 > q.config.MaxFiles {
		return fmt.Errorf("workspace file limit %d reached: %w", q.config.MaxFiles, api.ErrQuota)
	}
	if newFile {
		q.files += 1
	}
	return nil
}

func (q *quota) release(files int, bytes int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.scanned {
		return
	}
	q.files -= files
	q.bytes -= bytes
	if q.files < 0 || q.bytes < 0 {
		q.scanned = false
	}
}

func (q *quota) invalidate() {
	q.mu.Lock()
	q.scanned = false
	q.mu.Unlock()
}

func (q *quota) grow(n int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.config.MaxBytes > 0 && q.bytes+n > q.config.MaxBytes {
		return fmt.Errorf("workspace size limit %d bytes reached: %w", q.config.MaxBytes, api.ErrQuota)
	}
	q.bytes += n
	return nil
}

type meteredWriter struct {
	io.WriteCloser
	q    *quota
	name string
}

func (w meteredWriter) Write(p []byte) (int, error) {
	if err := w.q.grow(int64(len(p))); err != nil {
		return 0, fmt.Errorf("%q: %w", w.name, err)
	}
	n, err := w.WriteCloser.Write(p)
	if n < len(p) {
		w.q.release(0, int64(len(p)-n))
	}
	return n, err
}
