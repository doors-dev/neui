package files

import (
	"time"

	"neui.dev/api"
)

type Entry struct {
	Path    string    `json:"path" description:"pass to the other files tools"`
	Size    int64     `json:"size"`
	Dir     bool      `json:"dir,omitempty" description:"present and true for a directory"`
	MIME    string    `json:"mime,omitempty" description:"detected type, absent when unknown"`
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
