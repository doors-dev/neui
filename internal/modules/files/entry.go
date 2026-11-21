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
	"time"

	"neui.dev/api"
)

type Entry struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	Dir     bool      `json:"dir,omitempty"`
	MIME    string    `json:"mime,omitempty" description:"absent when unknown"`
	ModTime time.Time `json:"modTime,omitzero"`
}

type PathArg struct {
	Path string `json:"path" required:"true"`
}

func entry(e api.FSEntry) Entry {
	return Entry{
		Path:    e.Name,
		Size:    e.Size,
		Dir:     e.Dir,
		MIME:    e.MIME,
		ModTime: e.ModTime,
	}
}
