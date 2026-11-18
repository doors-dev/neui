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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/media"
)

const readImageDesc = `Read an image file as an image the model sees. PNG, JPEG, WebP or non-animated GIF; oversized images are downscaled.`
const readDocumentDesc = `Read a file as a document attachment, such as a PDF. Plain text formats are better read with the read tool.`

var documentExts = map[string]api.DocumentMime{
	".pdf":      api.DocumentPDF,
	".txt":      api.DocumentText,
	".json":     api.DocumentJSON,
	".md":       api.DocumentMarkdown,
	".markdown": api.DocumentMarkdown,
	".csv":      api.DocumentCSV,
	".html":     api.DocumentHTML,
	".htm":      api.DocumentHTML,
	".xml":      api.DocumentXML,
	".yaml":     api.DocumentYAML,
	".yml":      api.DocumentYAML,
	".rtf":      api.DocumentRTF,
	".doc":      api.DocumentDOC,
	".xls":      api.DocumentXLS,
	".docx":     api.DocumentDOCX,
	".xlsx":     api.DocumentXLSX,
	".pptx":     api.DocumentPPTX,
}

func readImageTool(ctx context.Context, arg PathArg) (api.Image, error) {
	thread := common.GetController(ctx)
	data, err := readCapped(ctx, thread.FS(), arg.Path, int64(thread.FilesConfig().ImageFileLimit))
	if err != nil {
		return api.Image{}, err
	}
	mimeType := api.ImageMime(http.DetectContentType(data))
	if !mimeType.IsValid() {
		return api.Image{}, fmt.Errorf("%q is %s, not a supported image", arg.Path, mimeType)
	}
	if mimeType == api.ImageGIF {
		if err := media.StillGIF(data); err != nil {
			return api.Image{}, fmt.Errorf("%q is %w", arg.Path, err)
		}
	}
	return media.Shape(data, mimeType, thread.Model())
}

func readDocumentTool(ctx context.Context, arg PathArg) (api.Document, error) {
	thread := common.GetController(ctx)
	data, err := readCapped(ctx, thread.FS(), arg.Path, int64(thread.FilesConfig().DocumentLimit))
	if err != nil {
		return api.Document{}, err
	}
	mimeType, ok := documentExts[strings.ToLower(path.Ext(arg.Path))]
	if !ok {
		t, _, _ := strings.Cut(http.DetectContentType(data), ";")
		mimeType = api.DocumentMime(strings.TrimSpace(t))
	}
	if !mimeType.IsValid() {
		return api.Document{}, fmt.Errorf("%q is %s, not a supported document", arg.Path, mimeType)
	}
	switch mimeType {
	case api.DocumentPDF:
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return api.Document{}, fmt.Errorf("%q is named as a PDF but does not start with %%PDF-", arg.Path)
		}
	case api.DocumentDOCX, api.DocumentXLSX, api.DocumentPPTX:
		if !bytes.HasPrefix(data, []byte("PK")) {
			return api.Document{}, fmt.Errorf("%q is named as %s but is not a ZIP package", arg.Path, path.Ext(arg.Path))
		}
	}
	return api.Document{Data: data, Mime: mimeType}, nil
}

func readCapped(ctx context.Context, fs api.FS, name string, cap int64) ([]byte, error) {
	rc, err := fs.ReadAll(ctx, name)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	if cap < 0 {
		return io.ReadAll(rc)
	}
	data, err := io.ReadAll(io.LimitReader(rc, cap+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > cap {
		return nil, fmt.Errorf("%q is over the %d-byte cap: %w", name, cap, api.ErrTooLarge)
	}
	return data, nil
}
