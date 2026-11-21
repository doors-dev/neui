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

package files

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"neui.dev/api"
	"neui.dev/internal/common"
)

const unzipListCap = 100

const zipDesc = `Zip files and directories into one archive, keeping their paths as given.`
const unzipDesc = `Extract a zip archive into a directory.`

type zipArg struct {
	Paths []string `json:"paths" required:"true" description:"walking a directory skips the uploads dirs, so list them here to include their files"`
	Dst   string   `json:"dst" required:"true" description:"path of the archive to create"`
}

type unzipArg struct {
	Src string `json:"src" required:"true"`
	Dst string `json:"dst" description:"absent or \"\" extracts into the workspace root"`
}

type unzipResult struct {
	_         struct{} `description:"count files were extracted, the first 100 listed in extracted; truncated: more exist"`
	Count     int      `json:"count"`
	Extracted []string `json:"extracted"`
	Truncated bool     `json:"truncated,omitempty"`
}

func zipTool(ctx context.Context, arg zipArg) (Entry, error) {
	fs := common.GetController(ctx).FS()
	out, err := fs.Create(arg.Dst)
	if err != nil {
		return Entry{}, err
	}
	created, err := fs.Stat(ctx, arg.Dst)
	if err != nil {
		out.Close()
		return Entry{}, errors.Join(err, fs.Remove(context.WithoutCancel(ctx), arg.Dst))
	}
	dst := created.Name
	zw := zip.NewWriter(out)
	added := map[string]bool{}
	err = func() error {
		for _, p := range arg.Paths {
			e, err := fs.Stat(ctx, p)
			if err != nil {
				return err
			}
			if !e.Dir {
				if e.Name == dst || added[e.Name] {
					continue
				}
				added[e.Name] = true
				if err := zipAdd(ctx, fs, zw, e.Name); err != nil {
					return err
				}
				continue
			}
			_, err = walkFiles(ctx, fs, e.Name, false, func(f api.FSEntry) (bool, error) {
				if f.Name == dst || added[f.Name] {
					return true, nil
				}
				added[f.Name] = true
				return true, zipAdd(ctx, fs, zw, f.Name)
			})
			if err != nil {
				return err
			}
		}
		return nil
	}()
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return Entry{}, errors.Join(err, fs.Remove(context.WithoutCancel(ctx), dst))
	}
	e, err := fs.Stat(ctx, dst)
	if err != nil {
		return Entry{}, err
	}
	return entry(e), nil
}

func zipAdd(ctx context.Context, fs api.FS, zw *zip.Writer, name string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	rc, err := fs.ReadAll(ctx, name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, ctxReader{ctx, rc})
	if cerr := rc.Close(); err == nil {
		err = cerr
	}
	return err
}

func unzipTool(ctx context.Context, arg unzipArg) (unzipResult, error) {
	thread := common.GetController(ctx)
	fs := thread.FS()
	config := thread.FilesConfig()
	total := config.UnzipLimit
	local, err := fs.LocalPath(ctx, arg.Src)
	if err != nil {
		return unzipResult{}, err
	}
	zr, err := zip.OpenReader(local)
	if err != nil {
		return unzipResult{}, err
	}
	defer zr.Close()
	res := unzipResult{Extracted: []string{}}
	var used int64
	for _, f := range zr.File {
		if err := context.Cause(ctx); err != nil {
			return res, err
		}
		if f.FileInfo().IsDir() || f.Mode()&os.ModeSymlink != 0 {
			continue
		}
		name := path.Clean(f.Name)
		if path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
			return res, fmt.Errorf("unsafe entry path %q in %q", f.Name, arg.Src)
		}
		if config.UnzipFiles > 0 && res.Count == config.UnzipFiles {
			return res, fmt.Errorf("%q holds over %d files", arg.Src, config.UnzipFiles)
		}
		budget := int64(-1)
		if total >= 0 {
			budget = total - used
		}
		target := path.Join(arg.Dst, name)
		n, err := unzipEntry(ctx, fs, f, target, budget)
		if err != nil {
			return res, err
		}
		used += n
		if total >= 0 && used > total {
			return res, errors.Join(fmt.Errorf("%q exceeds the %d-byte uncompressed cap", arg.Src, total), fs.Remove(ctx, target))
		}
		res.Count++
		if len(res.Extracted) < unzipListCap {
			res.Extracted = append(res.Extracted, target)
		} else {
			res.Truncated = true
		}
	}
	return res, nil
}

func unzipEntry(ctx context.Context, fs api.FS, f *zip.File, target string, budget int64) (int64, error) {
	zrc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer zrc.Close()
	rc := ctxReader{ctx, zrc}
	w, err := fs.Create(target)
	if err != nil {
		return 0, err
	}
	var n int64
	if budget < 0 {
		n, err = io.Copy(w, rc)
	} else {
		n, err = io.Copy(w, io.LimitReader(rc, budget+1))
	}
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return n, errors.Join(err, fs.Remove(context.WithoutCancel(ctx), target))
	}
	return n, nil
}

type ctxReader struct {
	ctx context.Context
	io.ReadCloser
}

func (r ctxReader) Read(p []byte) (int, error) {
	if err := context.Cause(r.ctx); err != nil {
		return 0, err
	}
	return r.ReadCloser.Read(p)
}
