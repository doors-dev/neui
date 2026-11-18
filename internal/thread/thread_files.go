package thread

import (
	"path"

	"github.com/doors-dev/doors"
	"neui.dev/api"
)

func uploadNames(attachments []string) []string {
	names := make([]string, len(attachments))
	for i, a := range attachments {
		names[i] = path.Base(a)
	}
	return names
}

func (t *thread) commitAttachments(input api.Input) {
	ctx := t.link.backend.Context()
	files := t.link.backend.UploadStorage()
	for _, name := range uploadNames(input.Attachments) {
		if err := files.FileCommit(ctx, t.id, api.OriginUser, name); err != nil {
			doors.Logger(ctx).Error("thread: attachment commit failed", "thread", t.id, "name", name, "err", err)
		}
	}
}

func (t *thread) rewindAttachments(removed []string, input api.Input) {
	ctx := t.link.backend.Context()
	files := t.link.backend.UploadStorage()
	for _, name := range uploadNames(removed) {
		if err := files.FileDelete(ctx, t.id, api.OriginUser, name); err != nil {
			doors.Logger(ctx).Error("thread: attachment delete failed", "thread", t.id, "name", name, "err", err)
		}
	}
	t.commitAttachments(input)
}

func (t *thread) sweepDrafts() error {
	return t.link.backend.UploadStorage().FileDiscardDrafts(t.link.backend.Context(), t.id, api.OriginUser, uploadNames(t.input.Attachments))
}
