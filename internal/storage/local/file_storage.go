package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"neui.dev/api"
)

const (
	dataName          = "data"
	metaName          = ".meta"
	filesDirName      = "files"
	workspacesDirName = "workspaces"
)

func NewFileStorage() (*FileStorage, error) {
	base, err := os.MkdirTemp("", "neui-files-")
	if err != nil {
		return nil, err
	}
	return &FileStorage{base: base}, nil
}

type FileStorage struct {
	mu   sync.Mutex
	base string
}

func validName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsRune(name, '/')
}

func checkFile(threadID api.ThreadID, origin api.Origin, name string) error {
	if !validName(string(threadID)) {
		return fmt.Errorf("%w: thread id %q", api.ErrInvalidName, threadID)
	}
	if !validName(name) {
		return fmt.Errorf("%w %q", api.ErrInvalidName, name)
	}
	if !origin.Valid() {
		return fmt.Errorf("unknown origin %q", origin)
	}
	return nil
}

func (l *FileStorage) FilePut(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string, mime string, r io.Reader, draft bool) (api.File, error) {
	if err := ctx.Err(); err != nil {
		return api.File{}, err
	}
	if err := checkFile(threadID, origin, name); err != nil {
		return api.File{}, err
	}
	threadDir := filepath.Join(l.base, filesDirName, string(threadID))
	originDir := filepath.Join(threadDir, string(origin))
	dst := filepath.Join(originDir, name)
	l.mu.Lock()
	_, statErr := os.Stat(filepath.Join(dst, metaName))
	l.mu.Unlock()
	if statErr == nil {
		return api.File{}, api.ErrExists
	}
	if err := os.MkdirAll(originDir, 0o755); err != nil {
		return api.File{}, err
	}
	stage, err := os.MkdirTemp(threadDir, ".tmp-")
	if err != nil {
		return api.File{}, err
	}
	f, err := stageFile(ctx, stage, api.File{
		ThreadID: threadID,
		Name:     name,
		MIME:     mime,
		Origin:   origin,
		Draft:    draft,
	}, r)
	if err != nil {
		os.RemoveAll(stage)
		return api.File{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := os.Stat(filepath.Join(dst, metaName)); err == nil {
		os.RemoveAll(stage)
		return api.File{}, api.ErrExists
	}
	if err := os.Rename(stage, dst); err != nil {
		os.RemoveAll(stage)
		if _, statErr := os.Stat(filepath.Join(dst, metaName)); statErr == nil {
			return api.File{}, api.ErrExists
		}
		return api.File{}, err
	}
	return f, nil
}

func stageFile(ctx context.Context, stage string, f api.File, r io.Reader) (api.File, error) {
	w, err := os.Create(filepath.Join(stage, dataName))
	if err != nil {
		return api.File{}, err
	}
	size, err := copyChunked(ctx, w, r)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return api.File{}, err
	}
	f.Size = size
	f.CreatedAt = time.Now()
	b, err := json.Marshal(f)
	if err != nil {
		return api.File{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, metaName), b, 0o644); err != nil {
		return api.File{}, err
	}
	return f, nil
}

func copyChunked(ctx context.Context, w io.Writer, r io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		k, err := r.Read(buf)
		if k > 0 {
			m, werr := w.Write(buf[:k])
			total += int64(m)
			if werr != nil {
				return total, werr
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

func readMeta(dir string) (api.File, error) {
	b, err := os.ReadFile(filepath.Join(dir, metaName))
	if errors.Is(err, fs.ErrNotExist) {
		return api.File{}, api.ErrNotFound
	}
	if err != nil {
		return api.File{}, err
	}
	var f api.File
	if err := json.Unmarshal(b, &f); err != nil {
		return api.File{}, err
	}
	return f, nil
}

func (l *FileStorage) FileCommit(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkFile(threadID, origin, name); err != nil {
		return err
	}
	dir := filepath.Join(l.base, filesDirName, string(threadID), string(origin), name)
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := readMeta(dir)
	if err != nil {
		return err
	}
	if !f.Draft {
		return nil
	}
	f.Draft = false
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, metaName+".tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, metaName))
}

func (l *FileStorage) FileDiscardDrafts(ctx context.Context, threadID api.ThreadID, origin api.Origin, keep []string) error {
	var names []string
	for f, err := range l.FileList(ctx, threadID, origin) {
		if err != nil {
			return err
		}
		if f.Draft && !slices.Contains(keep, f.Name) {
			names = append(names, f.Name)
		}
	}
	for _, name := range names {
		if err := l.FileDelete(ctx, threadID, origin, name); err != nil {
			return err
		}
	}
	return nil
}

func (l *FileStorage) FileStat(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string) (api.File, error) {
	if err := ctx.Err(); err != nil {
		return api.File{}, err
	}
	if err := checkFile(threadID, origin, name); err != nil {
		return api.File{}, err
	}
	dir := filepath.Join(l.base, filesDirName, string(threadID), string(origin), name)
	l.mu.Lock()
	defer l.mu.Unlock()
	return readMeta(dir)
}

func (l *FileStorage) FileRead(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string, off int64, n int64) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := checkFile(threadID, origin, name); err != nil {
		return nil, err
	}
	dir := filepath.Join(l.base, filesDirName, string(threadID), string(origin), name)
	l.mu.Lock()
	defer l.mu.Unlock()
	meta, err := readMeta(dir)
	if err != nil {
		return nil, err
	}
	if off < 0 {
		return nil, fmt.Errorf("negative offset %d", off)
	}
	if off >= meta.Size && (off > 0 || n >= 0) {
		return nil, fmt.Errorf("offset %d with size %d: %w", off, meta.Size, api.ErrOutOfRange)
	}
	f, err := os.Open(filepath.Join(dir, dataName))
	if err != nil {
		return nil, err
	}
	if off > 0 {
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			f.Close()
			return nil, err
		}
	}
	if n < 0 {
		return f, nil
	}
	return &limitReadCloser{Reader: io.LimitReader(f, n), file: f}, nil
}

type limitReadCloser struct {
	io.Reader
	file *os.File
}

func (l *limitReadCloser) Close() error {
	return l.file.Close()
}

func (l *FileStorage) FileList(ctx context.Context, threadID api.ThreadID, origin api.Origin) iter.Seq2[api.File, error] {
	return func(yield func(api.File, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(api.File{}, err)
			return
		}
		originDir := filepath.Join(l.base, filesDirName, string(threadID), string(origin))
		l.mu.Lock()
		entries, dirErr := os.ReadDir(originDir)
		var files []api.File
		var bad []error
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			entryDir := filepath.Join(originDir, e.Name())
			f, err := readMeta(entryDir)
			if errors.Is(err, api.ErrNotFound) {
				continue
			}
			if err != nil {
				bad = append(bad, err)
				continue
			}
			files = append(files, f)
		}
		l.mu.Unlock()
		if dirErr != nil && !errors.Is(dirErr, fs.ErrNotExist) {
			yield(api.File{}, dirErr)
			return
		}
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name < files[j].Name
		})
		for _, f := range files {
			if !yield(f, nil) {
				return
			}
		}
		for _, err := range bad {
			if !yield(api.File{}, err) {
				return
			}
		}
	}
}

func (l *FileStorage) FileDelete(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkFile(threadID, origin, name); err != nil {
		return err
	}
	dir := filepath.Join(l.base, filesDirName, string(threadID), string(origin), name)
	l.mu.Lock()
	defer l.mu.Unlock()
	return os.RemoveAll(dir)
}

func (l *FileStorage) FileDeleteAll(ctx context.Context, threadID api.ThreadID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validName(string(threadID)) {
		return fmt.Errorf("%w: thread id %q", api.ErrInvalidName, threadID)
	}
	threadDir := filepath.Join(l.base, filesDirName, string(threadID))
	workspaceDir := filepath.Join(l.base, workspacesDirName, string(threadID))
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.RemoveAll(threadDir); err != nil {
		return err
	}
	return os.RemoveAll(workspaceDir)
}

func (l *FileStorage) Remove() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return os.RemoveAll(l.base)
}

func (l *FileStorage) WorkspaceOpen(threadID api.ThreadID) (api.Workspace, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	dir := filepath.Join(l.base, workspacesDirName, string(threadID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return api.Workspace{}, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return api.Workspace{}, err
	}
	return api.Workspace{Dir: dir, Root: root}, nil
}

func (l *FileStorage) WorkspaceDelete(threadID api.ThreadID) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return os.RemoveAll(filepath.Join(l.base, workspacesDirName, string(threadID)))
}
