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
