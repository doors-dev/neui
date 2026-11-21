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

package xls

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"path"
	"strings"
	"sync"

	"github.com/xuri/excelize/v2"
	"neui.dev/api"
	"neui.dev/internal/filesystem"
)

type pathLock struct {
	mu   sync.Mutex
	refs int
}

var (
	locksMu sync.Mutex
	locks   = map[string]*pathLock{}
)

func openRead(ctx context.Context, fs api.FS, name string) (*excelize.File, error) {
	c := cfg(ctx)
	e, err := fs.Stat(ctx, name)
	if err != nil {
		return nil, err
	}
	if e.Size > c.MaxFileSize {
		return nil, fmt.Errorf("%q is %d bytes, over the module's cap of %d", name, e.Size, c.MaxFileSize)
	}
	path, err := fs.LocalPath(ctx, name)
	if err != nil {
		if errors.Is(err, api.ErrVirtual) {
			return nil, fmt.Errorf("%q is virtual — copy it into the workspace first (files__copy), then open the copy", name)
		}
		return nil, err
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid .xlsx workbook: %w", name, err)
	}
	var total uint64
	for _, zf := range z.File {
		total += zf.UncompressedSize64
		if zf.UncompressedSize64 > 1<<20 && zf.UncompressedSize64 > 100*zf.CompressedSize64 {
			z.Close()
			return nil, fmt.Errorf("part %q compresses %dx, over the 100x limit (likely a zip bomb)", zf.Name, zf.UncompressedSize64/max(zf.CompressedSize64, 1))
		}
	}
	z.Close()
	if total > uint64(c.MaxUnzipSize) {
		return nil, fmt.Errorf("%q expands to %d bytes uncompressed, over the module's cap of %d", name, total, c.MaxUnzipSize)
	}
	tmp, err := fs.TempDir()
	if err != nil {
		return nil, err
	}
	return excelize.OpenFile(path, excelize.Options{
		TmpDir:            tmp,
		UnzipSizeLimit:    c.MaxUnzipSize,
		UnzipXMLSizeLimit: min(16<<20, c.MaxUnzipSize),
	})
}

func openWrite(ctx context.Context, fs api.FS, name string, create bool) (*excelize.File, bool, func(), error) {
	tmp, err := fs.TempDir()
	if err != nil {
		return nil, false, nil, err
	}
	key := tmp + "\x00" + filesystem.ResolveName(name)
	locksMu.Lock()
	l := locks[key]
	if l == nil {
		l = &pathLock{}
		locks[key] = l
	}
	l.refs++
	locksMu.Unlock()
	l.mu.Lock()
	unlock := func() {
		l.mu.Unlock()
		locksMu.Lock()
		if l.refs--; l.refs == 0 {
			delete(locks, key)
		}
		locksMu.Unlock()
	}
	var f *excelize.File
	created := false
	_, statErr := fs.Stat(ctx, name)
	if !create && errors.Is(statErr, api.ErrNotFound) {
		unlock()
		return nil, false, nil, fmt.Errorf("%q: no workbook at this path: table or write creates one: %w", name, api.ErrNotFound)
	}
	if create && errors.Is(statErr, api.ErrNotFound) {
		if strings.ToLower(path.Ext(name)) != ".xlsx" {
			unlock()
			return nil, false, nil, fmt.Errorf("%q does not end in .xlsx: new workbooks are created as .xlsx only", name)
		}
		f, created = excelize.NewFile(), true
	} else if f, err = openRead(ctx, fs, name); err != nil {
		unlock()
		return nil, false, nil, err
	}
	if _, ok := f.Pkg.Load("xl/vbaProject.bin"); ok {
		f.Close()
		unlock()
		return nil, false, nil, fmt.Errorf("%q holds VBA macros, which this module does not support: it reads such a workbook but never writes it — build the report in a new .xlsx", name)
	}
	return f, created, func() {
		f.Close()
		unlock()
	}, nil
}

func save(ctx context.Context, fs api.FS, f *excelize.File, name string) error {
	tmp := fmt.Sprintf("%s.tmp-%016x~", name, rand.Uint64())
	w, err := fs.Create(tmp)
	if errors.Is(err, api.ErrReadOnly) {
		return fmt.Errorf("%q: %w", name, api.ErrReadOnly)
	}
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

func scanSheet(local, sheet string) (string, []string, error) {
	z, err := zip.OpenReader(local)
	if err != nil {
		return "", nil, err
	}
	defer z.Close()
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	var rels struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	for part, v := range map[string]any{"xl/workbook.xml": &wb, "xl/_rels/workbook.xml.rels": &rels} {
		r, err := z.Open(part)
		if err != nil {
			return "", nil, err
		}
		err = xml.NewDecoder(r).Decode(v)
		r.Close()
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", part, err)
		}
	}
	id := ""
	for _, s := range wb.Sheets {
		if strings.EqualFold(s.Name, sheet) {
			id = s.ID
		}
	}
	part := ""
	for _, rel := range rels.Rels {
		if id != "" && rel.ID == id {
			part = path.Join("xl", rel.Target)
			if strings.HasPrefix(rel.Target, "/") {
				part = strings.TrimPrefix(rel.Target, "/")
			}
		}
	}
	if part == "" {
		return "", nil, nil
	}
	r, err := z.Open(part)
	if err != nil {
		return "", nil, err
	}
	defer r.Close()
	filter := ""
	var arrays []string
	d := xml.NewDecoder(r)
	for {
		tok, err := d.RawToken()
		if err == io.EOF {
			return filter, arrays, nil
		}
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", part, err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "autoFilter" && se.Name.Local != "f" {
			continue
		}
		ref, array := "", false
		for _, a := range se.Attr {
			switch a.Name.Local {
			case "ref":
				ref = strings.ReplaceAll(a.Value, "$", "")
			case "t":
				array = a.Value == "array"
			}
		}
		if ref != "" && !strings.Contains(ref, ":") {
			ref += ":" + ref
		}
		if se.Name.Local == "autoFilter" {
			filter = ref
		} else if array && ref != "" {
			arrays = append(arrays, ref)
		}
	}
}
