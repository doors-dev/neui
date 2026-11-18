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

package nekit

import "neui.dev/nekit/internal/disk"

// DiskUploads keeps uploaded files in a directory on the local disk, where they
// survive restarts. Plug it in with neui.WithUploadStorage or
// neui.OptionUploadStorage.
type DiskUploads = *disk.Store

// NewDiskUploads stores files under path, creating it if missing; it fails
// when path is not a writable directory. Open a directory from one process
// only: opening discards unfinished uploads.
func NewDiskUploads(path string) (DiskUploads, error) {
	return disk.New(path)
}
