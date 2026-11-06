package uploader

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/kchat/params"
)

type refusingFS struct {
	api.FS
	err error
}

func (f refusingFS) UserUpload(ctx context.Context, name string, r io.Reader) (api.FSEntry, error) {
	return api.FSEntry{}, f.err
}

func filePart(t *testing.T) *multipart.Part {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("hello"))
	w.Close()
	part, err := multipart.NewReader(&body, w.Boundary()).NextPart()
	if err != nil {
		t.Fatal(err)
	}
	return part
}

func TestUploadRefusalStatus(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{api.ErrTooLarge, http.StatusRequestEntityTooLarge},
		{&http.MaxBytesError{Limit: 1}, http.StatusRequestEntityTooLarge},
		{api.ErrQuota, http.StatusUnprocessableEntity},
		{api.ErrExists, http.StatusUnprocessableEntity},
		{api.ErrInvalidName, http.StatusUnprocessableEntity},
		{errors.New("disk"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		ctx, cancel := context.WithCancel(context.Background())
		f := &fileProgress{
			fs:      refusingFS{err: c.err},
			door:    new(doors.Door),
			name:    "a.txt",
			percent: doors.NewSource(0),
			ctx:     ctx,
			cancel:  cancel,
			params:  params.Params{MaxAttachmentSize: 1 << 20},
			w:       rec,
		}
		added, err := f.download(context.Background(), filePart(t))
		if added || err == nil {
			t.Fatalf("%v: want a refused upload, got added=%v err=%v", c.err, added, err)
		}
		if rec.Code != c.status {
			t.Fatalf("%v: want status %d, got %d", c.err, c.status, rec.Code)
		}
	}
}
