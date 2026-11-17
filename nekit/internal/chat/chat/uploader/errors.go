package uploader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/params"
)

func describe(ctx context.Context, err error, p params.Params) string {
	var maxBytes *http.MaxBytesError
	if errors.Is(err, api.ErrTooMany) {
		return fmt.Sprintf("at most %d files", p.MaxAttachments)
	} else if errors.Is(err, api.ErrTooLarge) || errors.As(err, &maxBytes) {
		return "larger than " + humanSize(p.MaxAttachmentSize)
	} else if errors.Is(err, api.ErrQuota) {
		return "workspace is full"
	} else if errors.Is(err, api.ErrNotFound) {
		return "file is gone"
	} else if errors.Is(err, api.ErrExists) {
		return "name already used"
	} else if errors.Is(err, api.ErrInvalidName) {
		return "bad file name"
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "upload interrupted"
	}
	doors.Logger(ctx).Error("nechat: file error", "err", err)
	return "upload failed"
}

func humanSize(n int) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%d GB", n>>30)
	case n >= 1<<20:
		return fmt.Sprintf("%d MB", n>>20)
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}
