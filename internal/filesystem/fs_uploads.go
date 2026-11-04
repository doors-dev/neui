package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"

	"neui.dev/api"
)

func (w *workFS) readUpload(ctx context.Context, dir *uploadsDir, fileName string, off int64, n int64) (io.ReadCloser, error) {
	rc, err := w.files.FileRead(ctx, w.threadID, dir.origin, fileName, off, n)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", path.Join(dir.name, fileName), err)
	}
	return rc, nil
}

func (w *workFS) uploadCopy(ctx context.Context, dir *uploadsDir, src string, fileName string) error {
	srcFile, err := w.openRead(ctx, src)
	if err != nil {
		return notFound(src, err)
	}
	defer srcFile.Close()
	if f, ok := srcFile.(*os.File); ok {
		info, err := f.Stat()
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("%q: is a directory", src)
		}
	}
	_, err = w.files.FilePut(ctx, w.threadID, dir.origin, fileName, detectMIME(fileName), srcFile, false)
	if err != nil {
		return fmt.Errorf("%q: %w", path.Join(dir.name, fileName), err)
	}
	return nil
}

func (w *workFS) readUploadsDir(ctx context.Context, dir *uploadsDir) ([]api.FSEntry, error) {
	entries := make([]api.FSEntry, 0)
	for file, err := range w.files.FileList(ctx, w.threadID, dir.origin) {
		if err != nil {
			return nil, err
		}
		if file.Draft {
			continue
		}
		entries = append(entries, dir.entry(file))
	}
	return entries, nil
}

func (w *workFS) findUpload(ctx context.Context, dir *uploadsDir, fileName string) (api.File, error) {
	f, err := w.files.FileStat(ctx, w.threadID, dir.origin, fileName)
	if err == nil && f.Draft {
		err = api.ErrNotFound
	}
	if err != nil {
		return api.File{}, fmt.Errorf("%q: %w", path.Join(dir.name, fileName), err)
	}
	return f, nil
}

func displayName(name string) string {
	name = path.Base(name)
	if name == "." || name == ".." || name == "/" || name == "" {
		return "unnamed"
	}
	return name
}
