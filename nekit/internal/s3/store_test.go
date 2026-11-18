package s3

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"neui.dev/api"
)

const testBucket = "neui"

type fake struct {
	store       *Store
	client      *minio.Client
	backend     *s3mem.Backend
	completions atomic.Int32
}

func newFake(t *testing.T, prefix string) *fake {
	t.Helper()
	backend := s3mem.New()
	if err := backend.CreateBucket(testBucket); err != nil {
		t.Fatal(err)
	}
	f := &fake{backend: backend}
	server := gofakes3.New(backend).Server()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Query().Has("uploadId") {
			f.completions.Add(1)
			bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
			if r.Header.Get("If-None-Match") == "*" {
				if _, err := backend.HeadObject(bucket, key); err == nil {
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(http.StatusPreconditionFailed)
					fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>PreconditionFailed</Code><Message>At least one of the pre-conditions you specified did not hold</Message><Key>%s</Key></Error>`, key)
					return
				}
			}
		}
		server.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	client, err := minio.New(strings.TrimPrefix(srv.URL, "https://"), &minio.Options{
		Creds:     credentials.NewStaticV4("key", "secret", ""),
		Secure:    true,
		Transport: srv.Client().Transport,
		Region:    "us-east-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	f.client = client
	f.store = New(client, testBucket, prefix)
	return f
}

func newStore(t *testing.T) *Store {
	t.Helper()
	return newFake(t, "p").store
}

func (f *fake) keys(t *testing.T) []string {
	t.Helper()
	var keys []string
	for obj := range f.client.ListObjectsIter(context.Background(), testBucket, minio.ListObjectsOptions{Recursive: true}) {
		if obj.Err != nil {
			t.Fatal(obj.Err)
		}
		keys = append(keys, obj.Key)
	}
	return keys
}

func (f *fake) openUploads(t *testing.T) int {
	t.Helper()
	res, err := minio.Core{Client: f.client}.ListMultipartUploads(context.Background(), testBucket, "", "", "", "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	return len(res.Uploads)
}

func smallParts(t *testing.T) {
	t.Helper()
	old := partSize
	partSize = 5 << 20
	t.Cleanup(func() { partSize = old })
}

func putText(t *testing.T, s api.UploadStorage, thread api.ThreadID, name, body string) api.File {
	t.Helper()
	f, err := s.FilePut(context.Background(), thread, api.OriginUser, name, "text/plain", strings.NewReader(body), false)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func readAll(t *testing.T, rc io.ReadCloser) string {
	t.Helper()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func collect(t *testing.T, s api.UploadStorage, thread api.ThreadID, origin api.Origin) ([]api.File, []error) {
	t.Helper()
	var files []api.File
	var errs []error
	for f, err := range s.FileList(context.Background(), thread, origin) {
		if err != nil {
			errs = append(errs, err)
			continue
		}
		files = append(files, f)
	}
	return files, errs
}

type failReader struct{}

func (failReader) Read(p []byte) (int, error) {
	return 0, errors.New("broken body")
}

type raceReader struct {
	r    io.Reader
	race func()
	n    int
}

func (r *raceReader) Read(p []byte) (int, error) {
	if r.race != nil {
		r.race()
		r.race = nil
	}
	k, err := r.r.Read(p)
	r.n += k
	return k, err
}

type cutReader struct {
	r io.Reader
}

func (c cutReader) Read(p []byte) (int, error) {
	k, err := c.r.Read(p)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return k, err
}

func TestFilePutRoundTrip(t *testing.T) {
	s := newStore(t)
	f := putText(t, s, "t1", "f1.txt", "hello")
	if f.Size != 5 || f.CreatedAt.IsZero() || f.MIME != "text/plain" || f.Draft {
		t.Fatalf("file = %+v", f)
	}
	got, err := s.FileStat(context.Background(), "t1", api.OriginUser, "f1.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got.ThreadID != "t1" || got.Name != "f1.txt" || got.MIME != "text/plain" || got.Origin != api.OriginUser || got.Size != 5 || got.Draft {
		t.Fatalf("stat = %+v", got)
	}
	if !got.CreatedAt.Equal(f.CreatedAt) {
		t.Fatalf("CreatedAt = %v, want %v", got.CreatedAt, f.CreatedAt)
	}
	if d := time.Since(got.CreatedAt); d < -time.Minute || d > time.Minute {
		t.Fatalf("CreatedAt %v is not now", got.CreatedAt)
	}
	if body := readAll(t, must(s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", 0, -1))); body != "hello" {
		t.Fatalf("body = %q", body)
	}
}

func TestFilePutDuplicate(t *testing.T) {
	f := newFake(t, "p")
	s := f.store
	ctx := context.Background()
	putText(t, s, "t1", "f1.txt", "original")
	for _, draft := range []bool{false, true} {
		body := &raceReader{r: strings.NewReader("clobber")}
		if _, err := s.FilePut(ctx, "t1", api.OriginUser, "f1.txt", "", body, draft); !errors.Is(err, api.ErrExists) {
			t.Fatalf("draft %v: err = %v, want ErrExists", draft, err)
		}
		if body.n != 0 {
			t.Fatalf("draft %v: read %d bytes of the body before refusing a taken name", draft, body.n)
		}
		if st := must(s.FileStat(ctx, "t1", api.OriginUser, "f1.txt")); st.Draft {
			t.Fatalf("draft %v: committed file turned into a draft", draft)
		}
	}
	if body := readAll(t, must(s.FileRead(ctx, "t1", api.OriginUser, "f1.txt", 0, -1))); body != "original" {
		t.Fatalf("body = %q, want original untouched", body)
	}
	race := &raceReader{r: strings.NewReader("clobber"), race: func() { putText(t, s, "t1", "race.txt", "original") }}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "race.txt", "", race, false); !errors.Is(err, api.ErrExists) {
		t.Fatalf("racing put = %v, want ErrExists", err)
	}
	if body := readAll(t, must(s.FileRead(ctx, "t1", api.OriginUser, "race.txt", 0, -1))); body != "original" {
		t.Fatalf("body = %q, want the racing put's original", body)
	}
	if n := f.completions.Load(); n != 0 {
		t.Fatalf("small put went multipart: %d completions", n)
	}
}

func TestFilePutMultipart(t *testing.T) {
	smallParts(t)
	f := newFake(t, "p")
	s := f.store
	ctx := context.Background()
	for _, size := range []int{partSize, 2 * partSize, 2*partSize + 7} {
		body := make([]byte, size)
		rand.Read(body)
		name := fmt.Sprintf("big-%d.bin", size)
		got, err := s.FilePut(ctx, "t1", api.OriginAgent, name, "application/x-big", bytes.NewReader(body), false)
		if err != nil {
			t.Fatalf("FilePut %d: %v", size, err)
		}
		if got.Size != int64(size) || got.MIME != "application/x-big" {
			t.Fatalf("file = %+v", got)
		}
		stat, err := s.FileStat(ctx, "t1", api.OriginAgent, name)
		if err != nil {
			t.Fatal(err)
		}
		if stat.Size != int64(size) || stat.MIME != "application/x-big" || !stat.CreatedAt.Equal(got.CreatedAt) {
			t.Fatalf("stat = %+v, put = %+v", stat, got)
		}
		if read := readAll(t, must(s.FileRead(ctx, "t1", api.OriginAgent, name, 0, -1))); read != string(body) {
			t.Fatalf("%d bytes came back as %d, not the same", size, len(read))
		}
		if size == partSize {
			continue
		}
		if read := readAll(t, must(s.FileRead(ctx, "t1", api.OriginAgent, name, int64(partSize)-3, 6))); read != string(body[partSize-3:partSize+3]) {
			t.Fatalf("range across the part boundary = %x", read)
		}
	}
	if n := f.completions.Load(); n != 3 {
		t.Fatalf("completions = %d, want 3", n)
	}
	if n := f.openUploads(t); n != 0 {
		t.Fatalf("open uploads = %d", n)
	}
}

func TestFilePutMultipartDuplicate(t *testing.T) {
	smallParts(t)
	f := newFake(t, "p")
	s := f.store
	ctx := context.Background()
	body := bytes.Repeat([]byte("x"), partSize+1)
	race := &raceReader{r: bytes.NewReader(body), race: func() { putText(t, s, "t1", "f.bin", "original") }}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "f.bin", "", race, false); !errors.Is(err, api.ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if f.completions.Load() != 1 {
		t.Fatal("the duplicate did not go multipart")
	}
	if got := readAll(t, must(s.FileRead(ctx, "t1", api.OriginUser, "f.bin", 0, -1))); got != "original" {
		t.Fatalf("body = %d bytes, want original untouched", len(got))
	}
	if n := f.openUploads(t); n != 0 {
		t.Fatalf("open uploads after ErrExists = %d", n)
	}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "draft.bin", "", bytes.NewReader(body), true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "draft.bin", "", bytes.NewReader(body), true); !errors.Is(err, api.ErrExists) {
		t.Fatalf("draft duplicate = %v, want ErrExists", err)
	}
	if st := must(s.FileStat(ctx, "t1", api.OriginUser, "draft.bin")); !st.Draft || st.Size != int64(len(body)) {
		t.Fatalf("draft after a refused duplicate = %+v", st)
	}
	if n := f.openUploads(t); n != 0 {
		t.Fatalf("open uploads after draft ErrExists = %d", n)
	}
}

func TestFilePutMultipartFailureAborts(t *testing.T) {
	smallParts(t)
	f := newFake(t, "p")
	ctx := context.Background()
	body := io.MultiReader(bytes.NewReader(make([]byte, partSize+1)), failReader{})
	if _, err := f.store.FilePut(ctx, "t1", api.OriginUser, "f.bin", "", body, true); err == nil {
		t.Fatal("put with a body that breaks after one part succeeded")
	}
	if n := f.openUploads(t); n != 0 {
		t.Fatalf("open uploads = %d", n)
	}
	if keys := f.keys(t); len(keys) != 0 {
		t.Fatalf("left behind %v", keys)
	}
	putText(t, f.store, "t1", "f.bin", "retry")
}

func TestFilePutTruncatedBody(t *testing.T) {
	smallParts(t)
	f := newFake(t, "p")
	for _, size := range []int{100, partSize + 100} {
		if _, err := f.store.FilePut(context.Background(), "t1", api.OriginUser, "cut.bin", "", cutReader{bytes.NewReader(make([]byte, size))}, false); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("%d bytes cut short: err = %v, want io.ErrUnexpectedEOF", size, err)
		}
		if keys := f.keys(t); len(keys) != 0 {
			t.Fatalf("%d bytes cut short left behind %v", size, keys)
		}
	}
	if n := f.openUploads(t); n != 0 {
		t.Fatalf("open uploads = %d", n)
	}
}

func TestFilePutEmptyMIME(t *testing.T) {
	smallParts(t)
	s := newStore(t)
	ctx := context.Background()
	for _, size := range []int{1, partSize + 1} {
		name := fmt.Sprintf("f-%d", size)
		f, err := s.FilePut(ctx, "t1", api.OriginUser, name, "", bytes.NewReader(make([]byte, size)), false)
		if err != nil {
			t.Fatal(err)
		}
		if st := must(s.FileStat(ctx, "t1", api.OriginUser, name)); f.MIME != "" || st.MIME != "" {
			t.Fatalf("%d bytes: mime \"\" came back as put %q, stat %q", size, f.MIME, st.MIME)
		}
	}
	files, errs := collect(t, s, "t1", api.OriginUser)
	if len(errs) != 0 || len(files) != 2 || files[0].MIME != "" || files[1].MIME != "" {
		t.Fatalf("list = %+v %v", files, errs)
	}
}

func TestFilePutFailureFreesName(t *testing.T) {
	f := newFake(t, "p")
	ctx := context.Background()
	for _, draft := range []bool{false, true} {
		if _, err := f.store.FilePut(ctx, "t1", api.OriginUser, "f.txt", "", failReader{}, draft); err == nil {
			t.Fatal("put with broken body succeeded")
		}
		if keys := f.keys(t); len(keys) != 0 {
			t.Fatalf("draft %v left behind %v", draft, keys)
		}
	}
	putText(t, f.store, "t1", "f.txt", "retry")
}

func TestFileCommittedMarker(t *testing.T) {
	f := newFake(t, "p")
	s := f.store
	ctx := context.Background()
	marker := "p/t1/.committed/user/f.txt"
	if _, err := f.client.PutObject(ctx, testBucket, marker, bytes.NewReader(nil), 0, minio.PutObjectOptions{}); err != nil {
		t.Fatal(err)
	}
	if files, errs := collect(t, s, "t1", api.OriginUser); len(files) != 0 || len(errs) != 0 {
		t.Fatalf("marker without a file listed: %+v, %v", files, errs)
	}
	if _, err := s.FileStat(ctx, "t1", api.OriginUser, "f.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("marker without a file stat = %v, want ErrNotFound", err)
	}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "f.txt", "", failReader{}, false); err == nil {
		t.Fatal("put with broken body succeeded")
	}
	if _, err := f.client.StatObject(ctx, testBucket, marker, minio.StatObjectOptions{}); err != nil {
		t.Fatalf("failed plain put removed a marker it did not create: %v", err)
	}
	if got, err := s.FilePut(ctx, "t1", api.OriginUser, "f.txt", "text/plain", strings.NewReader("draft"), true); err != nil || !got.Draft {
		t.Fatalf("draft put over a stray marker = %+v, %v", got, err)
	}
	if st := must(s.FileStat(ctx, "t1", api.OriginUser, "f.txt")); !st.Draft {
		t.Fatal("draft put over a stray marker is not a draft")
	}
	if files, errs := collect(t, s, "t1", api.OriginUser); len(errs) != 0 || len(files) != 1 || !files[0].Draft {
		t.Fatalf("list = %+v, %v", files, errs)
	}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "f.txt", "text/plain", strings.NewReader("again"), true); !errors.Is(err, api.ErrExists) {
		t.Fatalf("second draft put = %v, want ErrExists", err)
	}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "f.txt", "text/plain", strings.NewReader("plain"), false); !errors.Is(err, api.ErrExists) {
		t.Fatalf("plain put over a draft = %v, want ErrExists", err)
	}
	if st := must(s.FileStat(ctx, "t1", api.OriginUser, "f.txt")); !st.Draft {
		t.Fatal("a refused put committed the draft")
	}
	stray := "p/t1/.committed/user/g.txt"
	if _, err := f.client.PutObject(ctx, testBucket, stray, bytes.NewReader(nil), 0, minio.PutObjectOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := putText(t, s, "t1", "g.txt", "plain"); got.Draft {
		t.Fatalf("plain put returned %+v", got)
	}
	if st := must(s.FileStat(ctx, "t1", api.OriginUser, "g.txt")); st.Draft {
		t.Fatal("plain put over a stray marker is a draft")
	}
	lost := &raceReader{r: strings.NewReader("plain"), race: func() {
		if _, err := s.FilePut(ctx, "t1", api.OriginUser, "h.txt", "", strings.NewReader("draft"), true); err != nil {
			t.Fatal(err)
		}
	}}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "h.txt", "", lost, false); !errors.Is(err, api.ErrExists) {
		t.Fatalf("plain put losing to a draft = %v, want ErrExists", err)
	}
	if st := must(s.FileStat(ctx, "t1", api.OriginUser, "h.txt")); !st.Draft {
		t.Fatal("a plain put that lost to a draft committed it")
	}
	lost = &raceReader{r: strings.NewReader("draft"), race: func() { putText(t, s, "t1", "i.txt", "plain") }}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "i.txt", "", lost, true); !errors.Is(err, api.ErrExists) {
		t.Fatalf("draft put losing to a plain put = %v, want ErrExists", err)
	}
	if st := must(s.FileStat(ctx, "t1", api.OriginUser, "i.txt")); st.Draft {
		t.Fatal("a draft put that lost to a plain put turned it into a draft")
	}
	want := "p/t1/.committed/user/g.txt,p/t1/.committed/user/i.txt,p/t1/user/f.txt,p/t1/user/g.txt,p/t1/user/h.txt,p/t1/user/i.txt"
	if keys := strings.Join(f.keys(t), ","); keys != want {
		t.Fatalf("keys = %s, want %s", keys, want)
	}
}

func TestFileAgentCommitted(t *testing.T) {
	f := newFake(t, "p")
	s := f.store
	ctx := context.Background()
	got, err := s.FilePut(ctx, "t1", api.OriginAgent, "a.txt", "text/plain", strings.NewReader("agent"), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Draft {
		t.Fatalf("put = %+v", got)
	}
	if st := must(s.FileStat(ctx, "t1", api.OriginAgent, "a.txt")); st.Draft {
		t.Fatal("agent file is a draft")
	}
	if err := s.FileDiscardDrafts(ctx, "t1", api.OriginAgent, nil); err != nil {
		t.Fatal(err)
	}
	files, errs := collect(t, s, "t1", api.OriginAgent)
	if len(errs) != 0 || len(files) != 1 || files[0].Draft || files[0].Name != "a.txt" {
		t.Fatalf("list = %+v, %v", files, errs)
	}
	if keys := strings.Join(f.keys(t), ","); keys != "p/t1/.committed/agent/a.txt,p/t1/agent/a.txt" {
		t.Fatalf("keys = %s", keys)
	}
}

func TestFileDraftCommit(t *testing.T) {
	f := newFake(t, "p")
	s := f.store
	ctx := context.Background()
	got, err := s.FilePut(ctx, "t1", api.OriginAgent, "d.txt", "text/plain", strings.NewReader("draft"), true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Draft {
		t.Fatalf("put = %+v", got)
	}
	files, errs := collect(t, s, "t1", api.OriginAgent)
	if len(errs) != 0 || len(files) != 1 || !files[0].Draft || files[0].MIME != "text/plain" || files[0].Size != 5 {
		t.Fatalf("list = %+v, %v", files, errs)
	}
	for range 2 {
		if err := s.FileCommit(ctx, "t1", api.OriginAgent, "d.txt"); err != nil {
			t.Fatal(err)
		}
		if st := must(s.FileStat(ctx, "t1", api.OriginAgent, "d.txt")); st.Draft {
			t.Fatal("still a draft after FileCommit")
		}
	}
	if err := s.FileCommit(ctx, "t1", api.OriginAgent, "ghost"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileCommit ghost = %v, want ErrNotFound", err)
	}
	if keys := f.keys(t); len(keys) != 2 || keys[0] != "p/t1/.committed/agent/d.txt" || keys[1] != "p/t1/agent/d.txt" {
		t.Fatalf("keys = %v", keys)
	}
}

func TestFileDiscardDrafts(t *testing.T) {
	f := newFake(t, "p")
	s := f.store
	ctx := context.Background()
	for _, name := range []string{"a", "b", "c"} {
		if _, err := s.FilePut(ctx, "t1", api.OriginUser, name, "text/plain", strings.NewReader(name), true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.FilePut(ctx, "t1", api.OriginAgent, "b", "text/plain", strings.NewReader("agent"), true); err != nil {
		t.Fatal(err)
	}
	putText(t, s, "t1", "plain", "x")
	if err := s.FileCommit(ctx, "t1", api.OriginUser, "a"); err != nil {
		t.Fatal(err)
	}
	if err := s.FileDiscardDrafts(ctx, "t1", api.OriginUser, []string{"c"}); err != nil {
		t.Fatal(err)
	}
	files, errs := collect(t, s, "t1", api.OriginUser)
	if len(errs) != 0 || len(files) != 3 || files[0].Name != "a" || files[1].Name != "c" || files[2].Name != "plain" {
		t.Fatalf("files = %+v, %v", files, errs)
	}
	if files[0].Draft || !files[1].Draft || files[2].Draft {
		t.Fatalf("draft flags = %+v", files)
	}
	if st := must(s.FileStat(ctx, "t1", api.OriginAgent, "b")); !st.Draft {
		t.Fatal("agent draft swept with the user's")
	}
	if err := s.FileDiscardDrafts(ctx, "ghost", api.OriginUser, nil); err != nil {
		t.Fatalf("missing thread = %v", err)
	}
	if err := s.FileDiscardDrafts(ctx, "", api.OriginUser, nil); !errors.Is(err, api.ErrInvalidName) {
		t.Fatalf("empty thread = %v, want ErrInvalidName", err)
	}
	if err := s.FileDiscardDrafts(ctx, "t1", api.Origin("x"), nil); err == nil {
		t.Fatal("unknown origin succeeded")
	}
}

func TestFileSweptMidUpload(t *testing.T) {
	smallParts(t)
	s := newStore(t)
	ctx := context.Background()
	for _, size := range []int{9, partSize + 1} {
		for _, draft := range []bool{true, false} {
			name := fmt.Sprintf("%d-%v", size, draft)
			body := &raceReader{r: bytes.NewReader(make([]byte, size)), race: func() {
				if err := s.FileDiscardDrafts(ctx, "t1", api.OriginUser, nil); err != nil {
					t.Fatal(err)
				}
			}}
			if _, err := s.FilePut(ctx, "t1", api.OriginUser, name, "", body, draft); err != nil {
				t.Fatal(err)
			}
			if st := must(s.FileStat(ctx, "t1", api.OriginUser, name)); st.Draft != draft || st.Size != int64(size) {
				t.Fatalf("%s swept while uploading: stat = %+v", name, st)
			}
		}
	}
}

func TestFileOriginsSeparate(t *testing.T) {
	s := newStore(t)
	putText(t, s, "t1", "f1.txt", "user side")
	if _, err := s.FilePut(context.Background(), "t1", api.OriginAgent, "f1.txt", "", strings.NewReader("agent side"), false); err != nil {
		t.Fatal(err)
	}
	if body := readAll(t, must(s.FileRead(context.Background(), "t1", api.OriginAgent, "f1.txt", 0, -1))); body != "agent side" {
		t.Fatalf("body = %q", body)
	}
	if err := s.FileDelete(context.Background(), "t1", api.OriginAgent, "f1.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FileStat(context.Background(), "t1", api.OriginUser, "f1.txt"); err != nil {
		t.Fatalf("user side gone with agent delete: %v", err)
	}
}

func TestFilePutInvalid(t *testing.T) {
	s := newStore(t)
	cases := []struct {
		thread api.ThreadID
		origin api.Origin
		name   string
	}{
		{"", api.OriginUser, "f1.txt"},
		{"t1", api.OriginUser, ""},
		{"t1", api.OriginUser, "a/b.txt"},
		{"t1", api.OriginUser, "."},
		{"t1", api.OriginUser, ".."},
		{"t1", api.Origin("smuggled"), "f1.txt"},
		{"t1", api.Origin(""), "f1.txt"},
	}
	for _, c := range cases {
		_, err := s.FilePut(context.Background(), c.thread, c.origin, c.name, "", strings.NewReader("x"), false)
		if err == nil {
			t.Fatalf("FilePut(%+v) succeeded", c)
		}
		if errors.Is(err, api.ErrExists) || errors.Is(err, api.ErrNotFound) {
			t.Fatalf("FilePut(%+v) = %v, want a plain error", c, err)
		}
	}
}

func TestFilePutRefusesNamesS3CannotHold(t *testing.T) {
	s := newStore(t)
	fits := 1024 - len("p/t1/.committed/user/")
	for _, name := range []string{strings.Repeat("a", fits+1), "a\x00b", "bad\xffutf8"} {
		if _, err := s.FilePut(context.Background(), "t1", api.OriginUser, name, "", strings.NewReader("x"), false); !errors.Is(err, api.ErrInvalidName) {
			t.Errorf("FilePut %.20q = %v, want ErrInvalidName", name, err)
		}
	}
	long := strings.Repeat("a", fits)
	if _, err := s.FilePut(context.Background(), "t1", api.OriginUser, long, "", strings.NewReader("x"), true); err != nil {
		t.Fatal(err)
	}
	if err := s.FileCommit(context.Background(), "t1", api.OriginUser, long); err != nil {
		t.Fatal(err)
	}
}

func TestFileZeroByte(t *testing.T) {
	s := newStore(t)
	f := putText(t, s, "t1", "empty", "")
	if f.Size != 0 {
		t.Fatalf("Size = %d", f.Size)
	}
	if body := readAll(t, must(s.FileRead(context.Background(), "t1", api.OriginUser, "empty", 0, -1))); body != "" {
		t.Fatalf("body = %q", body)
	}
}

func TestFileReadRange(t *testing.T) {
	s := newStore(t)
	putText(t, s, "t1", "f1.txt", "hello world")
	cases := []struct {
		off, n int64
		want   string
	}{
		{0, -1, "hello world"},
		{6, -1, "world"},
		{0, 5, "hello"},
		{6, 5, "world"},
		{6, 100, "world"},
		{10, 1, "d"},
		{3, 1 << 62, "lo world"},
		{0, 1<<63 - 1, "hello world"},
	}
	for _, c := range cases {
		rc, err := s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", c.off, c.n)
		if err != nil {
			t.Fatalf("FileRead(%d, %d) = %v", c.off, c.n, err)
		}
		if body := readAll(t, rc); body != c.want {
			t.Fatalf("FileRead(%d, %d) = %q, want %q", c.off, c.n, body, c.want)
		}
	}
	for _, c := range []struct{ off, n int64 }{{11, -1}, {11, 1}, {100, 5}, {11, 0}} {
		if _, err := s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", c.off, c.n); !errors.Is(err, api.ErrOutOfRange) {
			t.Fatalf("FileRead(%d, %d) = %v, want ErrOutOfRange", c.off, c.n, err)
		}
	}
	if _, err := s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", -3, 5); err == nil || errors.Is(err, api.ErrOutOfRange) {
		t.Fatalf("negative off = %v, want a plain error", err)
	}
	if body := readAll(t, must(s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", 3, 0))); body != "" {
		t.Fatalf("zero n body = %q", body)
	}
	putText(t, s, "t1", "empty", "")
	if body := readAll(t, must(s.FileRead(context.Background(), "t1", api.OriginUser, "empty", 0, -1))); body != "" {
		t.Fatalf("empty body = %q", body)
	}
	for _, c := range []struct{ off, n int64 }{{0, 1}, {0, 0}, {1, -1}, {0, 1<<63 - 1}} {
		if _, err := s.FileRead(context.Background(), "t1", api.OriginUser, "empty", c.off, c.n); !errors.Is(err, api.ErrOutOfRange) {
			t.Fatalf("empty FileRead(%d, %d) = %v, want ErrOutOfRange", c.off, c.n, err)
		}
	}
}

func TestFileMissing(t *testing.T) {
	s := newStore(t)
	if _, err := s.FileStat(context.Background(), "t1", api.OriginUser, "ghost"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileStat = %v, want ErrNotFound", err)
	}
	if _, err := s.FileRead(context.Background(), "t1", api.OriginUser, "ghost", 0, -1); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileRead = %v, want ErrNotFound", err)
	}
	if _, err := s.FileRead(context.Background(), "t1", api.OriginUser, "ghost", 5, 3); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ranged FileRead = %v, want ErrNotFound", err)
	}
	if err := s.FileDelete(context.Background(), "t1", api.OriginUser, "ghost"); err != nil {
		t.Fatalf("FileDelete = %v, want nil", err)
	}
	if err := s.FileDeleteAll(context.Background(), "ghost-thread"); err != nil {
		t.Fatalf("FileDeleteAll = %v, want nil", err)
	}
}

func TestFileDelete(t *testing.T) {
	f := newFake(t, "p")
	s := f.store
	ctx := context.Background()
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "f1.txt", "", strings.NewReader("hello"), true); err != nil {
		t.Fatal(err)
	}
	if err := s.FileDelete(ctx, "t1", api.OriginUser, "f1.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FileStat(ctx, "t1", api.OriginUser, "f1.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileStat after FileDelete = %v, want ErrNotFound", err)
	}
	putText(t, s, "t1", "f2.txt", "hello")
	if err := s.FileDelete(ctx, "t1", api.OriginUser, "f2.txt"); err != nil {
		t.Fatal(err)
	}
	if keys := f.keys(t); len(keys) != 0 {
		t.Fatalf("FileDelete left %v", keys)
	}
	putText(t, s, "t1", "keep", "x")
	putText(t, s, "t2", "f1.txt", "hello")
	if _, err := s.FilePut(ctx, "t2", api.OriginAgent, "f2.txt", "", strings.NewReader("hello"), true); err != nil {
		t.Fatal(err)
	}
	if err := s.FileDeleteAll(ctx, "t2"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FileStat(ctx, "t2", api.OriginUser, "f1.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileStat after FileDeleteAll = %v, want ErrNotFound", err)
	}
	if keys := f.keys(t); len(keys) != 2 || keys[0] != "p/t1/.committed/user/keep" || keys[1] != "p/t1/user/keep" {
		t.Fatalf("keys after FileDeleteAll = %v", keys)
	}
}

func TestFileListSorted(t *testing.T) {
	s := newStore(t)
	for _, name := range []string{"mango", "zebra", "apple", "Zulu", "é", "a b"} {
		putText(t, s, "t1", name, "1")
	}
	files, errs := collect(t, s, "t1", api.OriginUser)
	if len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	if got := strings.Join(names, ","); got != "Zulu,a b,apple,mango,zebra,é" {
		t.Fatalf("names = %s, want byte order", got)
	}
}

func TestFileListEmptyAndMissing(t *testing.T) {
	s := newStore(t)
	if files, errs := collect(t, s, "never-seen", api.OriginUser); len(files) != 0 || len(errs) != 0 {
		t.Fatalf("missing thread: files = %v, errs = %v", files, errs)
	}
	putText(t, s, "t1", "f1.txt", "x")
	if files, errs := collect(t, s, "t1", api.OriginAgent); len(files) != 0 || len(errs) != 0 {
		t.Fatalf("missing origin: files = %v, errs = %v", files, errs)
	}
	if err := s.FileDelete(context.Background(), "t1", api.OriginUser, "f1.txt"); err != nil {
		t.Fatal(err)
	}
	if files, errs := collect(t, s, "t1", api.OriginUser); len(files) != 0 || len(errs) != 0 {
		t.Fatalf("emptied origin: files = %v, errs = %v", files, errs)
	}
}

func TestFileListSkipsForeignKeys(t *testing.T) {
	f := newFake(t, "p")
	putText(t, f.store, "t1", "f1.txt", "x")
	for _, key := range []string{"p/t1/user/sub/deep.txt", "p/t1/userx", "p/t10/user/other"} {
		if _, err := f.client.PutObject(context.Background(), testBucket, key, bytes.NewReader(nil), 0, minio.PutObjectOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	files, errs := collect(t, f.store, "t1", api.OriginUser)
	if len(errs) != 0 || len(files) != 1 || files[0].Name != "f1.txt" {
		t.Fatalf("files = %+v, errs = %v, want only f1.txt", files, errs)
	}
}

func TestFileListError(t *testing.T) {
	f := newFake(t, "p")
	s := New(f.client, "no-such-bucket", "")
	_, errs := collect(t, s, "t1", api.OriginUser)
	if len(errs) != 1 || errors.Is(errs[0], api.ErrNotFound) {
		t.Fatalf("errs = %v, want one listing error", errs)
	}
}

func TestPrefix(t *testing.T) {
	for prefix, want := range map[string]string{"": "", "/": "", "a": "a/", "a/": "a/", "a/b//": "a/b/"} {
		f := newFake(t, prefix)
		putText(t, f.store, "t1", "f", "x")
		if keys := f.keys(t); len(keys) != 2 || keys[0] != want+"t1/.committed/user/f" || keys[1] != want+"t1/user/f" {
			t.Fatalf("prefix %q: keys = %v, want %s", prefix, keys, want)
		}
		if err := f.store.FileDeleteAll(context.Background(), "t1"); err != nil {
			t.Fatal(err)
		}
		if keys := f.keys(t); len(keys) != 0 {
			t.Fatalf("prefix %q: left %v", prefix, keys)
		}
	}
}

func TestFileCancelledContext(t *testing.T) {
	s := newStore(t)
	putText(t, s, "t1", "f1.txt", "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "f2.txt", "", strings.NewReader("x"), false); err == nil {
		t.Fatal("FilePut with cancelled ctx succeeded")
	}
	if _, err := s.FileStat(ctx, "t1", api.OriginUser, "f1.txt"); err == nil {
		t.Fatal("FileStat with cancelled ctx succeeded")
	}
	if _, err := s.FileRead(ctx, "t1", api.OriginUser, "f1.txt", 0, -1); err == nil {
		t.Fatal("FileRead with cancelled ctx succeeded")
	}
	if err := s.FileCommit(ctx, "t1", api.OriginUser, "f1.txt"); err == nil {
		t.Fatal("FileCommit with cancelled ctx succeeded")
	}
	if err := s.FileDiscardDrafts(ctx, "t1", api.OriginUser, nil); err == nil {
		t.Fatal("FileDiscardDrafts with cancelled ctx succeeded")
	}
	if err := s.FileDelete(ctx, "t1", api.OriginUser, "f1.txt"); err == nil {
		t.Fatal("FileDelete with cancelled ctx succeeded")
	}
	if err := s.FileDeleteAll(ctx, "t1"); err == nil {
		t.Fatal("FileDeleteAll with cancelled ctx succeeded")
	}
	var errs []error
	for _, err := range s.FileList(ctx, "t1", api.OriginUser) {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		t.Fatal("FileList with cancelled ctx yielded no error")
	}
	if _, err := s.FileStat(context.Background(), "t1", api.OriginUser, "f1.txt"); err != nil {
		t.Fatalf("f1.txt gone: %v", err)
	}
}

func TestFileMethodsRejectInvalidNames(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	putText(t, s, "t1", "f1.txt", "hello")
	for _, name := range []string{"", ".", "..", "/", "a/b"} {
		if err := s.FileDelete(ctx, "t1", api.OriginUser, name); !errors.Is(err, api.ErrInvalidName) {
			t.Fatalf("FileDelete(%q) = %v, want ErrInvalidName", name, err)
		}
		if _, err := s.FileStat(ctx, "t1", api.OriginUser, name); !errors.Is(err, api.ErrInvalidName) {
			t.Fatalf("FileStat(%q) = %v, want ErrInvalidName", name, err)
		}
		if _, err := s.FileRead(ctx, "t1", api.OriginUser, name, 0, -1); !errors.Is(err, api.ErrInvalidName) {
			t.Fatalf("FileRead(%q) = %v, want ErrInvalidName", name, err)
		}
		if err := s.FileCommit(ctx, "t1", api.OriginUser, name); !errors.Is(err, api.ErrInvalidName) {
			t.Fatalf("FileCommit(%q) = %v, want ErrInvalidName", name, err)
		}
	}
	for _, thread := range []api.ThreadID{"", ".", "..", "a/b"} {
		if err := s.FileDelete(ctx, thread, api.OriginUser, "f1.txt"); !errors.Is(err, api.ErrInvalidName) {
			t.Fatalf("FileDelete in thread %q = %v, want ErrInvalidName", thread, err)
		}
		if err := s.FileDeleteAll(ctx, thread); !errors.Is(err, api.ErrInvalidName) {
			t.Fatalf("FileDeleteAll(%q) = %v, want ErrInvalidName", thread, err)
		}
		for _, err := range s.FileList(ctx, thread, api.OriginUser) {
			if !errors.Is(err, api.ErrInvalidName) {
				t.Fatalf("FileList(%q) = %v, want ErrInvalidName", thread, err)
			}
		}
	}
	if err := s.FileDelete(ctx, "t1", api.Origin("smuggled"), "f1.txt"); err == nil {
		t.Fatal("FileDelete with an unknown origin succeeded")
	}
	listed := false
	for _, err := range s.FileList(ctx, "t1", api.Origin("..")) {
		if err == nil {
			t.Fatal("FileList with an unknown origin yielded a file")
		}
		listed = true
	}
	if !listed {
		t.Fatal("FileList with an unknown origin yielded no error")
	}
	if got := readAll(t, must(s.FileRead(ctx, "t1", api.OriginUser, "f1.txt", 0, -1))); got != "hello" {
		t.Fatalf("upload changed to %q", got)
	}
}

func TestLive(t *testing.T) {
	endpoint, access, secret, bucket := os.Getenv("NEUI_S3_ENDPOINT"), os.Getenv("NEUI_S3_ACCESS_KEY"), os.Getenv("NEUI_S3_SECRET_KEY"), os.Getenv("NEUI_S3_BUCKET")
	if endpoint == "" || access == "" || secret == "" || bucket == "" {
		t.Skip("set NEUI_S3_ENDPOINT, NEUI_S3_ACCESS_KEY, NEUI_S3_SECRET_KEY and NEUI_S3_BUCKET to run against a real service")
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(access, secret, ""),
		Secure: os.Getenv("NEUI_S3_INSECURE") != "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := New(client, bucket, fmt.Sprintf("neui-test-%d", time.Now().UnixNano()))
	ctx := context.Background()
	t.Cleanup(func() {
		if err := s.FileDeleteAll(context.Background(), "t1"); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	putText(t, s, "t1", "small.txt", "hello")
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "small.txt", "", strings.NewReader("clobber"), false); !errors.Is(err, api.ErrExists) {
		t.Fatalf("single put over a taken name = %v, want ErrExists", err)
	}
	smallParts(t)
	body := make([]byte, partSize+1)
	rand.Read(body)
	if _, err := s.FilePut(ctx, "t1", api.OriginAgent, "big.bin", "application/octet-stream", bytes.NewReader(body), true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FilePut(ctx, "t1", api.OriginAgent, "big.bin", "", bytes.NewReader(make([]byte, partSize+1)), false); !errors.Is(err, api.ErrExists) {
		t.Fatalf("multipart put over a taken name = %v, want ErrExists", err)
	}
	if got := readAll(t, must(s.FileRead(ctx, "t1", api.OriginAgent, "big.bin", 0, -1))); got != string(body) {
		t.Fatal("multipart body changed or did not round-trip")
	}
	uploads, err := minio.Core{Client: client}.ListMultipartUploads(ctx, bucket, s.prefix, "", "", "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(uploads.Uploads) != 0 {
		t.Fatalf("open uploads = %d", len(uploads.Uploads))
	}
	if st := must(s.FileStat(ctx, "t1", api.OriginAgent, "big.bin")); !st.Draft || st.Size != int64(len(body)) {
		t.Fatalf("stat = %+v", st)
	}
	if err := s.FileCommit(ctx, "t1", api.OriginAgent, "big.bin"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FileRead(ctx, "t1", api.OriginUser, "small.txt", 5, 1); !errors.Is(err, api.ErrOutOfRange) {
		t.Fatalf("read past the end = %v, want ErrOutOfRange", err)
	}
	files, errs := collect(t, s, "t1", api.OriginAgent)
	if len(errs) != 0 || len(files) != 1 || files[0].Draft {
		t.Fatalf("list = %+v, %v", files, errs)
	}
}
