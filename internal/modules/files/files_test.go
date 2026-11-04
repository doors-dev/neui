package files

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/testkit"
)

func putFile(t *testing.T, store *local.FileStorage, origin api.Origin, name, content string) {
	t.Helper()
	if _, err := store.FilePut(context.Background(), "thread", origin, name, "text/plain", strings.NewReader(content), false); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, fs api.FS, name, content string) {
	t.Helper()
	w, err := fs.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestModuleBuilds(t *testing.T) {
	m := Module(context.Background())
	count := 0
	for range m.List() {
		count++
	}
	if count != 19 {
		t.Fatalf("tools = %d, want 19", count)
	}
}

func TestReadPaging(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	var b strings.Builder
	for i := 1; i <= 5; i++ {
		b.WriteString(strings.Repeat("x", 10))
		b.WriteString("\n")
	}
	write(t, fs, "a.txt", b.String())
	read := readTool
	out, err := read(c.Ctx(), readArg{Path: "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "1: xxxxxxxxxx\n") || !strings.HasSuffix(out, "(end of file, 5 lines)") {
		t.Fatalf("out = %q", out)
	}
	out, err = read(c.Ctx(), readArg{Path: "a.txt", Offset: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2: ") || !strings.Contains(out, "3: ") || !strings.Contains(out, "(showing lines 2-3 of 5, continue with offset=4)") {
		t.Fatalf("out = %q", out)
	}
	if _, err := read(c.Ctx(), readArg{Path: "a.txt", Offset: 9}); err == nil {
		t.Fatal("offset past EOF succeeded")
	}
	write(t, fs, "empty.txt", "")
	out, err = read(c.Ctx(), readArg{Path: "empty.txt"})
	if err != nil || out != "(empty file)" {
		t.Fatalf("empty: %q, %v", out, err)
	}
	write(t, fs, "bin", "a\x00b")
	if _, err := read(c.Ctx(), readArg{Path: "bin"}); err == nil || !strings.Contains(err.Error(), "readBytes") {
		t.Fatalf("binary: %v", err)
	}
}

func TestReadByteLimit(t *testing.T) {
	limited := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{ReadLimit: 25}})
	c := testkit.NewController(t, testkit.Setup{})
	var b strings.Builder
	for i := 1; i <= 5; i++ {
		b.WriteString(strings.Repeat("x", 10))
		b.WriteString("\n")
	}
	write(t, limited.FS(), "a.txt", b.String())
	write(t, c.FS(), "a.txt", b.String())
	out, err := readTool(limited.Ctx(), readArg{Path: "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "3: xxxxxxxxxx") || strings.Contains(out, "4: ") {
		t.Fatalf("out = %q", out)
	}
	if !strings.HasSuffix(out, "(showing lines 1-3 of 5, continue with offset=4)") {
		t.Fatalf("out = %q", out)
	}
	out, err = readTool(c.Ctx(), readArg{Path: "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out, "(end of file, 5 lines)") {
		t.Fatalf("default limit truncated: %q", out)
	}
}

func TestReadBytes(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "b.bin", "hello world")
	rb := readBytesTool
	data, err := rb(c.Ctx(), readBytesArg{Path: "b.bin", Offset: 6, N: 5})
	if err != nil || string(data) != "world" {
		t.Fatalf("%q, %v", data, err)
	}
	data, err = rb(c.Ctx(), readBytesArg{Path: "b.bin"})
	if err != nil || string(data) != "hello world" {
		t.Fatalf("%q, %v", data, err)
	}
}

func TestWriteBytes(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	e, err := writeBytesTool(c.Ctx(), writeBytesArg{Path: "out/x.bin", Data: []byte{1, 2, 3}})
	if err != nil || e.Size != 3 || e.Path != "out/x.bin" {
		t.Fatalf("%+v, %v", e, err)
	}
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadImage(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	small := pngBytes(t, 10, 10)
	if _, err := writeBytesTool(c.Ctx(), writeBytesArg{Path: "small.png", Data: small}); err != nil {
		t.Fatal(err)
	}
	img, err := readImageTool(c.Ctx(), PathArg{Path: "small.png"})
	if err != nil || img.Mime != api.ImagePNG || !bytes.Equal(img.Data, small) {
		t.Fatalf("mime %s, equal %v, %v", img.Mime, bytes.Equal(img.Data, small), err)
	}
	write(t, fs, "not.png", "just text")
	if _, err := readImageTool(c.Ctx(), PathArg{Path: "not.png"}); err == nil {
		t.Fatal("text accepted as image")
	}
}

func TestReadImageDownscales(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	big := pngBytes(t, 4000, 100)
	if _, err := writeBytesTool(c.Ctx(), writeBytesArg{Path: "big.png", Data: big}); err != nil {
		t.Fatal(err)
	}
	img, err := readImageTool(c.Ctx(), PathArg{Path: "big.png"})
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width > 2048 || cfg.Height > 2048 {
		t.Fatalf("dims %dx%d", cfg.Width, cfg.Height)
	}
}

func TestReadImageUsesModelPixels(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Model: api.Model{ImagePixels: 1568 * 1568}})
	big := pngBytes(t, 4000, 3000)
	if _, err := writeBytesTool(c.Ctx(), writeBytesArg{Path: "wide.png", Data: big}); err != nil {
		t.Fatal(err)
	}
	img, err := readImageTool(c.Ctx(), PathArg{Path: "wide.png"})
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width*cfg.Height > 1568*1568 {
		t.Fatalf("%dx%d is %d pixels, over %d", cfg.Width, cfg.Height, cfg.Width*cfg.Height, 1568*1568)
	}
}

func TestReadDocument(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "doc.pdf", "%PDF-1.4 fake")
	doc, err := readDocumentTool(c.Ctx(), PathArg{Path: "doc.pdf"})
	if err != nil || doc.Mime != api.DocumentPDF {
		t.Fatalf("mime %s, %v", doc.Mime, err)
	}
	write(t, fs, "prog.bin", "\x00\x01\x02")
	if _, err := readDocumentTool(c.Ctx(), PathArg{Path: "prog.bin"}); err == nil {
		t.Fatal("binary accepted as document")
	}
}

func TestZipUnzipRoundTrip(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "dir/a.txt", "alpha")
	write(t, fs, "dir/sub/b.txt", "beta")
	e, err := zipTool(c.Ctx(), zipArg{Paths: []string{"dir"}, Dst: "out.zip"})
	if err != nil || e.Path != "out.zip" || e.Size == 0 {
		t.Fatalf("%+v, %v", e, err)
	}
	res, err := unzipTool(c.Ctx(), unzipArg{Src: "out.zip", Dst: "restored"})
	if err != nil || res.Count != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
	rc, err := fs.ReadAll(context.Background(), "restored/dir/sub/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "beta" {
		t.Fatalf("got %q", got)
	}
}

func TestZipWalkSkipsUploads(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs, store := c.FS(), c.Store()
	write(t, fs, "dir/a.txt", "alpha")
	putFile(t, store, api.OriginUser, "up.txt", "upload")
	e, err := zipTool(c.Ctx(), zipArg{Paths: []string{"."}, Dst: "out.zip"})
	if err != nil || e.Size == 0 {
		t.Fatalf("%+v, %v", e, err)
	}
	names := zipNames(t, fs, "out.zip")
	if len(names) != 1 || names[0] != "dir/a.txt" {
		t.Fatalf("names = %v", names)
	}
	e, err = zipTool(c.Ctx(), zipArg{Paths: []string{"user_uploads"}, Dst: "up.zip"})
	if err != nil || e.Size == 0 {
		t.Fatalf("%+v, %v", e, err)
	}
	names = zipNames(t, fs, "up.zip")
	if len(names) != 1 || names[0] != "user_uploads/up.txt" {
		t.Fatalf("names = %v", names)
	}
}

func zipNames(t *testing.T, fs api.FS, name string) []string {
	t.Helper()
	local, err := fs.LocalPath(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(local)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

func TestUnzipUploadNeedsCopy(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	store := c.Store()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("inner.txt")
	io.WriteString(w, "x")
	zw.Close()
	putFile(t, store, api.OriginUser, "a.zip", buf.String())
	if _, err := unzipTool(c.Ctx(), unzipArg{Src: "user_uploads/a.zip", Dst: "out"}); err == nil || !strings.Contains(err.Error(), "copy") {
		t.Fatalf("upload src: %v", err)
	}
	if _, err := copyTool(c.Ctx(), copyArg{Src: "user_uploads/a.zip", Dst: "a.zip"}); err != nil {
		t.Fatal(err)
	}
	res, err := unzipTool(c.Ctx(), unzipArg{Src: "a.zip", Dst: "out"})
	if err != nil || res.Count != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
}

func TestUnzipWithTheCapTurnedOff(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{UnzipLimit: -1}})
	fs := c.FS()
	ctx := c.Ctx()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("inner.txt")
	io.WriteString(w, "payload")
	zw.Close()
	if _, err := writeBytesTool(ctx, writeBytesArg{Path: "a.zip", Data: buf.Bytes()}); err != nil {
		t.Fatal(err)
	}
	write(t, fs, "out/inner.txt", "the file that was already there")
	res, err := unzipTool(ctx, unzipArg{Src: "a.zip", Dst: "out"})
	if err != nil || res.Count != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	rc, err := fs.ReadAll(context.Background(), "out/inner.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "payload" {
		t.Fatalf("extracted %q, want %q: the entry read as empty and the create truncated the target", got, "payload")
	}
}

func TestMediaCapsRefuseWithErrTooLarge(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{DocumentLimit: 4, ImageFileLimit: 4}})
	fs := c.FS()
	write(t, fs, "doc.pdf", "%PDF-1.4 fake")
	ctx := c.Ctx()
	if _, err := readDocumentTool(ctx, PathArg{Path: "doc.pdf"}); !errors.Is(err, api.ErrTooLarge) {
		t.Fatalf("a document over DocumentLimit: want api.ErrTooLarge, got %v", err)
	}
	if _, err := readImageTool(ctx, PathArg{Path: "doc.pdf"}); !errors.Is(err, api.ErrTooLarge) {
		t.Fatalf("a file over ImageFileLimit: want api.ErrTooLarge, got %v", err)
	}
}

func TestUnzipRejectsEscape(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("../evil.txt")
	io.WriteString(w, "x")
	zw.Close()
	if _, err := writeBytesTool(c.Ctx(), writeBytesArg{Path: "evil.zip", Data: buf.Bytes()}); err != nil {
		t.Fatal(err)
	}
	if _, err := unzipTool(c.Ctx(), unzipArg{Src: "evil.zip"}); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("err = %v", err)
	}
}

func TestGrep(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "a.go", "package a\nfunc Hello() {}\n")
	write(t, fs, "b.txt", "hello there\n")
	res, err := grepTool(c.Ctx(), grepArg{Pattern: "(?i)hello"})
	if err != nil || len(res.Matches) != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
	res, err = grepTool(c.Ctx(), grepArg{Pattern: "(?i)hello", Glob: "**/*.go"})
	if err != nil || len(res.Matches) != 1 || res.Matches[0].Path != "a.go" || res.Matches[0].Line != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
}

func TestGrepSkipsUploadsUnlessPathed(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs, store := c.FS(), c.Store()
	write(t, fs, "local.txt", "needle here\n")
	putFile(t, store, api.OriginUser, "up.txt", "needle in upload\n")
	putFile(t, store, api.OriginAgent, "mem.txt", "needle in memory\n")
	res, err := grepTool(c.Ctx(), grepArg{Pattern: "needle"})
	if err != nil || len(res.Matches) != 1 || res.Matches[0].Path != "local.txt" {
		t.Fatalf("default walk: %+v, %v", res, err)
	}
	res, err = grepTool(c.Ctx(), grepArg{Pattern: "needle", Path: "user_uploads"})
	if err != nil || len(res.Matches) != 1 || res.Matches[0].Path != "user_uploads/up.txt" {
		t.Fatalf("explicit dir: %+v, %v", res, err)
	}
	res, err = grepTool(c.Ctx(), grepArg{Pattern: "needle", Path: "agent_uploads/mem.txt"})
	if err != nil || len(res.Matches) != 1 || res.Matches[0].Path != "agent_uploads/mem.txt" {
		t.Fatalf("explicit file: %+v, %v", res, err)
	}
}

func TestGlobListsUploads(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs, store := c.FS(), c.Store()
	write(t, fs, "top.csv", "1")
	putFile(t, store, api.OriginUser, "data.csv", "2")
	res, err := globTool(c.Ctx(), globArg{Pattern: "**/*.csv"})
	if err != nil || len(res.Paths) != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
}

func TestGlob(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "x/deep/data.csv", "1")
	write(t, fs, "top.csv", "2")
	write(t, fs, "note.txt", "3")
	res, err := globTool(c.Ctx(), globArg{Pattern: "**/*.csv"})
	if err != nil || len(res.Paths) != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
}

func TestAgentUploadReplaceLoop(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "MEMORY.md", "v1")
	cp := copyTool
	del := deleteTool
	if _, err := cp(c.Ctx(), copyArg{Src: "MEMORY.md", Dst: "agent_uploads/MEMORY.md"}); err != nil {
		t.Fatal(err)
	}
	write(t, fs, "MEMORY.md", "v2")
	if _, err := cp(c.Ctx(), copyArg{Src: "MEMORY.md", Dst: "agent_uploads/MEMORY.md"}); !errors.Is(err, api.ErrExists) {
		t.Fatalf("overwrite without delete: %v", err)
	}
	if _, err := del(c.Ctx(), PathArg{Path: "agent_uploads/MEMORY.md"}); err != nil {
		t.Fatal(err)
	}
	if _, err := cp(c.Ctx(), copyArg{Src: "MEMORY.md", Dst: "agent_uploads/MEMORY.md"}); err != nil {
		t.Fatal(err)
	}
	rc, err := fs.ReadAll(context.Background(), "agent_uploads/MEMORY.md")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "v2" {
		t.Fatalf("got %q", got)
	}
}

func TestCopyToAgentUploadsSurfacesExists(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "r.txt", "data")
	cp := copyTool
	e, err := cp(c.Ctx(), copyArg{Src: "r.txt", Dst: "agent_uploads/r.txt"})
	if err != nil || e.Path != "agent_uploads/r.txt" {
		t.Fatalf("%+v, %v", e, err)
	}
	if _, err := cp(c.Ctx(), copyArg{Src: "r.txt", Dst: "agent_uploads/r.txt"}); !errors.Is(err, api.ErrExists) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadOverlongLine(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "a.txt", "first\r\n"+strings.Repeat("x", 3<<20)+"\r\nthird\n")
	out, err := readTool(c.Ctx(), readArg{Path: "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	want := "1: first\n2: " + strings.Repeat("x", readLineMax) + "... (line truncated)\n3: third\n(end of file, 3 lines)"
	if out != want {
		t.Fatalf("got %.200q", out)
	}
}

func TestGrepOverlongLine(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "a/big.txt", strings.Repeat("y", 3<<20)+"\nneedle one\n")
	write(t, fs, "b/small.txt", "needle two\n")
	res, err := grepTool(c.Ctx(), grepArg{Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 2 || res.Matches[0].Line != 2 {
		t.Fatalf("got %+v", res.Matches)
	}
}

func TestZipSkipsExplicitDst(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "a.txt", "a")
	write(t, fs, "out.zip", "old")
	if _, err := zipTool(c.Ctx(), zipArg{Paths: []string{"a.txt", "out.zip"}, Dst: "out.zip"}); err != nil {
		t.Fatal(err)
	}
	if names := zipNames(t, fs, "out.zip"); len(names) != 1 || names[0] != "a.txt" {
		t.Fatalf("got %v", names)
	}
}

func TestReadImageRejectsOversizedGIF(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	header := []byte("GIF89a\xff\xff\xff\xff\x00\x00\x00;")
	if _, err := writeBytesTool(c.Ctx(), writeBytesArg{Path: "huge.gif", Data: header}); err != nil {
		t.Fatal(err)
	}
	_, err := readImageTool(c.Ctx(), PathArg{Path: "huge.gif"})
	if err == nil || !strings.Contains(err.Error(), "65535x65535") {
		t.Fatalf("err %v", err)
	}
}
