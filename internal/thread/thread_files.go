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
