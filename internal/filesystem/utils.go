package filesystem

import (
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"

	"neui.dev/api"
)

type uploadsDir struct {
	name   string
	origin api.Origin
}

var (
	userUploads  = &uploadsDir{name: uploadsDirName, origin: api.OriginUser}
	agentUploads = &uploadsDir{name: agentUploadsDirName, origin: api.OriginAgent}
	uploadsDirs  = []*uploadsDir{userUploads, agentUploads}
)

func (w *workFS) resolveUpload(name string) (clean string, dir *uploadsDir, fileName string, err error) {
	name = path.Clean(name)
	lower, temp := strings.ToLower(name), strings.ToLower(w.temp)
	if lower == temp || strings.HasPrefix(lower, temp+"/") {
		return name, nil, "", fmt.Errorf("%q: %w", name, api.ErrReserved)
	}
	for _, d := range uploadsDirs {
		if strings.EqualFold(name, d.name) {
			return name, d, "", nil
		}
		if len(name) <= len(d.name) || name[len(d.name)] != '/' || !strings.EqualFold(name[:len(d.name)], d.name) {
			continue
		}
		rest := name[len(d.name)+1:]
		if strings.Contains(rest, "/") {
			return name, d, "", fmt.Errorf(
				"%q: %s/ is flat and contains no subdirectories; files are directly inside it, e.g. %s/%s: %w",
				name, d.name, d.name, path.Base(rest), api.ErrInvalidName)
		}
		return name, d, rest, nil
	}
	return name, nil, "", nil
}

func (d *uploadsDir) entry(f api.File) api.FSEntry {
	return api.FSEntry{
		Name:    path.Join(d.name, f.Name),
		Size:    f.Size,
		ModTime: f.CreatedAt,
		MIME:    f.MIME,
		Virtual: true,
	}
}

func isUploadsDirName(name string) bool {
	for _, d := range uploadsDirs {
		if strings.EqualFold(name, d.name) {
			return true
		}
	}
	return false
}

func detectMIME(name string) string {
	return cleanMIME(mime.TypeByExtension(path.Ext(name)))
}

func sniffMIME(head []byte) string {
	return cleanMIME(http.DetectContentType(head))
}

func cleanMIME(t string) string {
	t, _, _ = strings.Cut(t, ";")
	return strings.TrimSpace(t)
}
