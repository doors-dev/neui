package xls

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/xuri/excelize/v2"
	"neui.dev/api"
)

func openRead(ctx context.Context, fs api.FS, name string) (*excelize.File, error) {
	c := cfg(ctx)
	e, err := fs.Stat(ctx, name)
	if err != nil {
		return nil, err
	}
	if e.Size > c.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes, limit %d", e.Size, c.MaxFileSize)
	}
	path, err := fs.LocalPath(ctx, name)
	if err != nil {
		if errors.Is(err, api.ErrVirtual) {
			return nil, fmt.Errorf("%q is virtual — copy it into the workspace first (files__copy), then open the copy", name)
		}
		return nil, err
	}
	tmp, err := fs.TempDir()
	if err != nil {
		return nil, err
	}
	return excelize.OpenFile(path, excelize.Options{
		TmpDir:            tmp,
		UnzipSizeLimit:    4 * c.MaxFileSize,
		UnzipXMLSizeLimit: 16 << 20,
	})
}

func save(ctx context.Context, fs api.FS, f *excelize.File, name string) error {
	tmp := fmt.Sprintf("%s.tmp-%016x~", name, rand.Uint64())
	w, err := fs.Create(tmp)
	if err != nil {
		return err
	}
	err = f.Write(w)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = fs.Rename(tmp, name)
	}
	if err != nil {
		fs.Remove(ctx, tmp)
		return err
	}
	return nil
}
