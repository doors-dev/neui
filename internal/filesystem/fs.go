package filesystem

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"neui.dev/api"
)

const uploadsDirName = api.UserUploadsDir
const agentUploadsDirName = api.AgentUploadsDir
const tempDirPrefix = ".tmp-"

type FS = *workFS

func New(threadID api.ThreadID, workspace api.Workspace, files api.FileStorage, config api.FilesConfig) FS {
	return &workFS{
		threadID:  threadID,
		workspace: workspace,
		files:     files,
		temp:      tempDirPrefix + string(threadID),
		quota:     &quota{config: config},
	}
}

type workFS struct {
	threadID  api.ThreadID
	workspace api.Workspace
	files     api.FileStorage
	temp      string
	quota     *quota
	mu        sync.Mutex
}

func (w *workFS) Append(name string) (io.WriteCloser, error) {
	return w.openWrite(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY)
}

func (w *workFS) Copy(ctx context.Context, src string, dst string) error {
	src, _, _, err := w.resolveUpload(src)
	if err != nil {
		return err
	}
	dst, dir, fileName, err := w.resolveUpload(dst)
	if err != nil {
		return err
	}
	switch dir {
	case nil:
	case userUploads:
		return fmt.Errorf("%q: %w", dst, api.ErrReadOnly)
	case agentUploads:
		if fileName == "" {
			return fmt.Errorf("%q: is a directory", dst)
		}
		return w.uploadCopy(ctx, dir, src, fileName)
	default:
		panic("unknown uploads dir")
	}
	srcFile, err := w.openRead(ctx, src)
	if err != nil {
		return notFound(src, err)
	}
	defer srcFile.Close()
	if f, ok := srcFile.(*os.File); ok {
		srcInfo, err := f.Stat()
		if err != nil {
			return err
		}
		if srcInfo.IsDir() {
			return fmt.Errorf("%q: is a directory", src)
		}
		if dstInfo, err := w.workspace.Root.Stat(dst); err == nil && os.SameFile(srcInfo, dstInfo) {
			return nil
		}
	}
	dstFile, err := w.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(dstFile, srcFile)
	if cerr := dstFile.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		w.removeLocal(dst)
		return err
	}
	return nil
}

func (w *workFS) Create(name string) (io.WriteCloser, error) {
	return w.openWrite(name, os.O_TRUNC|os.O_CREATE|os.O_WRONLY)
}

func (w *workFS) UploadStat(ctx context.Context, name string) (api.FSEntry, error) {
	name, dir, fileName, err := w.resolveUpload(name)
	if err != nil {
		return api.FSEntry{}, err
	}
	if dir == nil || fileName == "" {
		return api.FSEntry{}, fmt.Errorf("%q: not an upload: %w", name, api.ErrInvalidName)
	}
	f, err := w.files.FileStat(ctx, w.threadID, dir.origin, fileName)
	if err != nil {
		return api.FSEntry{}, fmt.Errorf("%q: %w", name, err)
	}
	return dir.entry(f), nil
}

func (w *workFS) DeleteUpload(ctx context.Context, name string) error {
	name, dir, fileName, err := w.resolveUpload(name)
	if err != nil {
		return err
	}
	if dir == nil || fileName == "" {
		return fmt.Errorf("%q: not an upload: %w", name, api.ErrInvalidName)
	}
	return w.removeUpload(ctx, dir, fileName)
}

func (w *workFS) removeUpload(ctx context.Context, dir *uploadsDir, fileName string) error {
	return w.files.FileDelete(ctx, w.threadID, dir.origin, fileName)
}

func (w *workFS) LocalPath(ctx context.Context, name string) (string, error) {
	name, dir, _, err := w.resolveUpload(name)
	if err != nil {
		return "", err
	}
	if dir != nil {
		return "", fmt.Errorf("%q: %w", name, api.ErrVirtual)
	}
	if _, err := w.workspace.Root.Stat(name); err != nil {
		return "", notFound(name, err)
	}
	return filepath.Join(w.workspace.Dir, name), nil
}

func (w *workFS) TempDir() (string, error) {
	if err := w.workspace.Root.MkdirAll(w.temp, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(w.workspace.Dir, w.temp), nil
}

func (w *workFS) TempWipe() error {
	return w.workspace.Root.RemoveAll(w.temp)
}

func (w *workFS) ReadAll(ctx context.Context, name string) (io.ReadCloser, error) {
	name, _, _, err := w.resolveUpload(name)
	if err != nil {
		return nil, err
	}
	f, err := w.openRead(ctx, name)
	if err != nil {
		return nil, notFound(name, err)
	}
	return f, nil
}

func (w *workFS) ReadDir(ctx context.Context, name string) ([]api.FSEntry, error) {
	name, dir, fileName, err := w.resolveUpload(name)
	if err != nil {
		return nil, err
	}
	if dir != nil && fileName != "" {
		if _, err := w.findUpload(ctx, dir, fileName); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%q: not a directory", name)
	}
	if dir != nil {
		return w.readUploadsDir(ctx, dir)
	}
	f, err := w.workspace.Root.Open(name)
	if err != nil {
		return nil, notFound(name, err)
	}
	defer f.Close()
	list, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	entries := make([]api.FSEntry, 0, len(list)+len(uploadsDirs))
	if name == "." {
		for _, d := range uploadsDirs {
			entries = append(entries, api.FSEntry{Name: d.name, Dir: true, Virtual: true})
		}
	}
	for _, de := range list {
		if name == "." && (isUploadsDirName(de.Name()) || strings.EqualFold(de.Name(), w.temp)) {
			continue
		}
		info, err := de.Info()
		if err != nil {
			return nil, err
		}
		entry := api.FSEntry{
			Name:    path.Join(name, de.Name()),
			Size:    info.Size(),
			Dir:     de.IsDir(),
			ModTime: info.ModTime(),
		}
		if !entry.Dir {
			entry.MIME = detectMIME(entry.Name)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (w *workFS) ReadRange(ctx context.Context, name string, off int64, n int64) (io.ReadCloser, error) {
	name, dir, fileName, err := w.resolveUpload(name)
	if err != nil {
		return nil, err
	}
	if off < 0 {
		return nil, fmt.Errorf("%q: negative offset %d", name, off)
	}
	if dir != nil && fileName == "" {
		return nil, fmt.Errorf("%q: is a directory", dir.name)
	}
	if dir != nil {
		return w.readUpload(ctx, dir, fileName, off, n)
	}
	f, err := w.workspace.Root.Open(name)
	if err != nil {
		return nil, notFound(name, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if off >= info.Size() && (off > 0 || n >= 0) {
		f.Close()
		return nil, fmt.Errorf("%q: offset %d with size %d: %w", name, off, info.Size(), api.ErrOutOfRange)
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		f.Close()
		return nil, err
	}
	if n < 0 {
		return f, nil
	}
	return &rangeReadCloser{Reader: io.LimitReader(f, n), file: f}, nil
}

func (w *workFS) Remove(ctx context.Context, name string) error {
	name, dir, fileName, err := w.resolveUpload(name)
	if err != nil {
		return err
	}
	switch dir {
	case nil:
		return w.removeLocal(name)
	case userUploads:
		return fmt.Errorf("%q: %w", name, api.ErrReadOnly)
	case agentUploads:
		if fileName == "" {
			return fmt.Errorf("%q: %w", name, api.ErrReadOnly)
		}
		return w.removeUpload(ctx, dir, fileName)
	default:
		panic("unknown uploads dir")
	}
}

func (w *workFS) Rename(oldname string, newname string) error {
	for _, name := range []*string{&oldname, &newname} {
		clean, dir, _, err := w.resolveUpload(*name)
		if err != nil {
			return err
		}
		if dir != nil {
			return fmt.Errorf("%q: %w", clean, api.ErrReadOnly)
		}
		*name = clean
	}
	overwritten, overwrittenSize := false, int64(0)
	if w.quota.active() {
		w.mu.Lock()
		defer w.mu.Unlock()
		if info, err := w.workspace.Root.Stat(newname); err == nil && !info.IsDir() {
			if old, err := w.workspace.Root.Stat(oldname); err != nil || !os.SameFile(old, info) {
				overwritten, overwrittenSize = true, info.Size()
			}
		}
	}
	err := w.workspace.Root.Rename(oldname, newname)
	if err == nil {
		if overwritten {
			w.quota.release(1, overwrittenSize)
		}
		return nil
	}
	if _, serr := w.workspace.Root.Stat(oldname); serr != nil {
		return notFound(oldname, serr)
	}
	return notFound(newname, err)
}

func (w *workFS) Stat(ctx context.Context, name string) (api.FSEntry, error) {
	name, dir, fileName, err := w.resolveUpload(name)
	if err != nil {
		return api.FSEntry{}, err
	}
	if dir != nil && fileName == "" {
		return api.FSEntry{Name: dir.name, Dir: true, Virtual: true}, nil
	}
	if dir != nil {
		file, err := w.findUpload(ctx, dir, fileName)
		if err != nil {
			return api.FSEntry{}, err
		}
		return dir.entry(file), nil
	}
	info, err := w.workspace.Root.Stat(name)
	if err != nil {
		return api.FSEntry{}, notFound(name, err)
	}
	entry := api.FSEntry{
		Name:    name,
		Size:    info.Size(),
		Dir:     info.IsDir(),
		ModTime: info.ModTime(),
	}
	if !entry.Dir {
		entry.MIME = detectMIME(name)
	}
	return entry, nil
}

func (w *workFS) UserUpload(ctx context.Context, name string, r io.Reader) (api.FSEntry, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return api.FSEntry{}, err
	}
	display := displayName(name)
	ext := path.Ext(display)
	base := strings.TrimSuffix(display, ext)
	mime := sniffMIME(head[:n])
	body := io.MultiReader(bytes.NewReader(head[:n]), r)
	fileName := display
	for i := 1; ; i++ {
		file, err := w.files.FilePut(ctx, w.threadID, userUploads.origin, fileName, mime, body, true)
		if err == nil {
			return userUploads.entry(file), nil
		}
		if !errors.Is(err, api.ErrExists) {
			return api.FSEntry{}, err
		}
		fileName = fmt.Sprintf("%s(%d)%s", base, i, ext)
	}
}

func (w *workFS) openWrite(name string, flag int) (io.WriteCloser, error) {
	name, dir, _, err := w.resolveUpload(name)
	if err != nil {
		return nil, err
	}
	if dir != nil {
		return nil, fmt.Errorf("%q: %w", name, api.ErrReadOnly)
	}
	if dir := path.Dir(name); dir != "." {
		if err := w.workspace.Root.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	if !w.quota.active() {
		return w.workspace.Root.OpenFile(name, flag, 0o644)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.quota.ensure(w.workspace.Root.FS(), w.temp); err != nil {
		return nil, err
	}
	var oldSize int64
	existed := false
	if info, err := w.workspace.Root.Stat(name); err == nil && !info.IsDir() {
		existed, oldSize = true, info.Size()
	}
	if err := w.quota.admit(!existed); err != nil {
		return nil, fmt.Errorf("%q: %w", name, err)
	}
	f, err := w.workspace.Root.OpenFile(name, flag, 0o644)
	if err != nil {
		if !existed {
			w.quota.release(1, 0)
		}
		return nil, err
	}
	if existed && flag&os.O_TRUNC != 0 {
		w.quota.release(0, oldSize)
	}
	return meteredWriter{WriteCloser: f, q: w.quota, name: name}, nil
}

func (w *workFS) removeLocal(name string) error {
	if !w.quota.active() {
		return w.workspace.Root.RemoveAll(name)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.workspace.Root.Stat(name); err != nil {
		return w.workspace.Root.RemoveAll(name)
	}
	if err := w.workspace.Root.RemoveAll(name); err != nil {
		return err
	}
	w.quota.invalidate()
	return nil
}

func (w *workFS) openRead(ctx context.Context, name string) (io.ReadCloser, error) {
	name, dir, fileName, err := w.resolveUpload(name)
	if err != nil {
		return nil, err
	}
	if dir != nil && fileName == "" {
		return nil, fmt.Errorf("%q: is a directory", dir.name)
	}
	if dir != nil {
		return w.readUpload(ctx, dir, fileName, 0, -1)
	}
	return w.workspace.Root.Open(name)
}

func notFound(name string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%q: %w", name, api.ErrNotFound)
	}
	return err
}

type rangeReadCloser struct {
	io.Reader
	file *os.File
}

func (r *rangeReadCloser) Close() error {
	return r.file.Close()
}

var _ FS = (FS)(nil)
