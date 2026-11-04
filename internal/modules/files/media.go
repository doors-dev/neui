package files

import (
	"bytes"
	"context"
	"fmt"
	"image/gif"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/media"
)

const readImageDesc = `Read an image file as an image the model sees. PNG, JPEG, WebP or non-animated GIF; oversized images are downscaled.`
const readDocumentDesc = `Read a file as a document attachment, such as a PDF. Plain text formats are better read with the read tool.`

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
		g, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return api.Image{}, err
		}
		if len(g.Image) > 1 {
			return api.Image{}, fmt.Errorf("%q is an animated GIF, which providers do not accept", arg.Path)
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
	mimeType := docMime(arg.Path, data)
	if !mimeType.IsValid() {
		return api.Document{}, fmt.Errorf("%q is %s, not a supported document", arg.Path, mimeType)
	}
	return api.Document{Data: data, Mime: mimeType}, nil
}

func docMime(name string, data []byte) api.DocumentMime {
	t := mime.TypeByExtension(path.Ext(name))
	if t == "" {
		t = http.DetectContentType(data)
	}
	t, _, _ = strings.Cut(t, ";")
	return api.DocumentMime(strings.TrimSpace(t))
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
