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
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"slices"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/testkit"
	"neui.dev/internal/tools"
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
	if n := len(Module(context.WithValue(context.Background(), common.TierKey{}, common.TierAgent)).List()); n != 19 {
		t.Fatalf("TierAgent registers %d tools, want 19", n)
	}
	var names []string
	for _, tool := range Module(context.WithValue(context.Background(), common.TierKey{}, common.TierAssistant)).List() {
		names = append(names, tool.Name())
	}
	if want := []string{"copy", "delete", "edit", "grep", "list", "read", "readDocument", "readImage", "write"}; !slices.Equal(names, want) {
		t.Fatalf("TierAssistant registers %v, want %v", names, want)
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
	if _, err := read(context.WithValue(c.Ctx(), common.TierKey{}, common.TierAgent), readArg{Path: "bin"}); err == nil || !strings.Contains(err.Error(), "readBytes") {
		t.Fatalf("binary at TierAgent: %v", err)
	}
	if _, err := read(context.WithValue(c.Ctx(), common.TierKey{}, common.TierAssistant), readArg{Path: "bin"}); err == nil || strings.Contains(err.Error(), "readBytes") || !strings.Contains(err.Error(), "readDocument") {
		t.Fatalf("binary at TierAssistant: %v", err)
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

func TestWriteBytesErrorsNameThePath(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	_, err := writeBytesTool(c.Ctx(), writeBytesArg{Path: "agent_uploads/x.bin", Data: []byte{1}})
	if !errors.Is(err, api.ErrReadOnly) || !strings.HasPrefix(err.Error(), `"agent_uploads/x.bin": `) {
		t.Fatalf("%v", err)
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

func TestMediaBudgetRefusesADocumentOverIt(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Config: api.ThreadConfig{ThreadMemoryLimit: 100, ReplayMediaShare: 0.2}})
	fs := c.FS()
	write(t, fs, "doc.pdf", "%PDF-1.4 over ten bytes")
	write(t, fs, "small.pdf", "%PDF-1.4")
	ctx := c.Ctx()
	_, err := readDocumentTool(ctx, PathArg{Path: "doc.pdf"})
	if !errors.Is(err, api.ErrTooLarge) || !strings.Contains(err.Error(), "more than a turn's tool results can hold") {
		t.Fatalf("a document over the media budget: want api.ErrTooLarge saying a turn cannot hold it, got %v", err)
	}
	if _, err := readDocumentTool(ctx, PathArg{Path: "small.pdf"}); err != nil {
		t.Fatalf("a document under the media budget: %v", err)
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

func TestGlobRelativeToPath(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "src/a.go", "package a\n")
	write(t, fs, "src/sub/b.go", "package b\n")
	write(t, fs, "c.go", "package c\n")
	res, err := globTool(c.Ctx(), globArg{Pattern: "*.go", Path: "src"})
	if err != nil || len(res.Paths) != 1 || res.Paths[0] != "src/a.go" {
		t.Fatalf("%+v, %v", res, err)
	}
	res, err = globTool(c.Ctx(), globArg{Pattern: "sub/*.go", Path: "src/"})
	if err != nil || len(res.Paths) != 1 || res.Paths[0] != "src/sub/b.go" {
		t.Fatalf("%+v, %v", res, err)
	}
	res, err = globTool(c.Ctx(), globArg{Pattern: "*.go"})
	if err != nil || len(res.Paths) != 1 || res.Paths[0] != "c.go" {
		t.Fatalf("%+v, %v", res, err)
	}
	gres, err := grepTool(c.Ctx(), grepArg{Pattern: "package", Path: "src", Glob: "*.go"})
	if err != nil || len(gres.Matches) != 1 || gres.Matches[0].Path != "src/a.go" {
		t.Fatalf("%+v, %v", gres, err)
	}
	gres, err = grepTool(c.Ctx(), grepArg{Pattern: "package", Path: "src/sub/b.go", Glob: "*.go"})
	if err != nil || len(gres.Matches) != 1 || gres.Matches[0].Path != "src/sub/b.go" {
		t.Fatalf("%+v, %v", gres, err)
	}
}

func TestReadDocumentExtensionsAndMagic(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	cases := []struct {
		name, content string
		mime          api.DocumentMime
	}{
		{"notes.md", "# hi", api.DocumentMarkdown},
		{"notes.MARKDOWN", "# hi", api.DocumentMarkdown},
		{"conf.yml", "a: 1", api.DocumentYAML},
		{"page.htm", "<p>x</p>", api.DocumentHTML},
		{"data.csv", "a,b", api.DocumentCSV},
		{"old.xls", "\xd0\xcf\x11\xe0", api.DocumentXLS},
		{"sheet.xlsx", "PK\x03\x04", api.DocumentXLSX},
		{"noext", "plain words", api.DocumentText},
	}
	for _, c := range cases {
		write(t, fs, c.name, c.content)
		doc, err := readDocumentTool(ctl.Ctx(), PathArg{Path: c.name})
		if err != nil || doc.Mime != c.mime {
			t.Fatalf("%s: mime %s, %v", c.name, doc.Mime, err)
		}
	}
	for _, name := range []string{"fake.pdf", "fake.docx", "fake.pptx"} {
		write(t, fs, name, "not the real thing")
		if _, err := readDocumentTool(ctl.Ctx(), PathArg{Path: name}); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func gifBytes(t *testing.T, frames int) []byte {
	t.Helper()
	g := &gif.GIF{}
	for range frames {
		g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White}))
		g.Delay = append(g.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadImageGIFFrames(t *testing.T) {
	ctx := testkit.NewController(t, testkit.Setup{}).Ctx()
	single := gifBytes(t, 1)
	many := []byte("GIF89a\x04\x00\x04\x00\x00\x00\x00")
	for range 100000 {
		many = append(many, 0x21, 0xF9, 4, 0, 10, 0, 0, 0)
		many = append(many, 0x2C, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0, 2, 2, 0x4C, 0x01, 0)
	}
	many = append(many, 0x3B)
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"single.gif", single, ""},
		{"anim.gif", gifBytes(t, 3), "animated"},
		{"many.gif", many, "animated"},
		{"cut.gif", single[:len(single)-3], "truncated"},
	}
	for _, c := range cases {
		if _, err := writeBytesTool(ctx, writeBytesArg{Path: c.name, Data: c.data}); err != nil {
			t.Fatal(err)
		}
		img, err := readImageTool(ctx, PathArg{Path: c.name})
		if c.want == "" {
			if err != nil || img.Mime != api.ImageGIF {
				t.Fatalf("%s: mime %s, %v", c.name, img.Mime, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: err %v", c.name, err)
		}
	}
}

func TestReadHidesTheBOMAndAgreesWithGrepOnLoneCR(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "b.txt", "\ufeffa\rb\r\nneedle\n")
	out, err := readTool(c.Ctx(), readArg{Path: "b.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "1: a\rb\n2: needle\n(end of file, 2 lines)"; out != want {
		t.Fatalf("got %q", out)
	}
	res, err := grepTool(c.Ctx(), grepArg{Pattern: "needle", Path: "b.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 || res.Matches[0].Line != 2 {
		t.Fatalf("got %+v", res.Matches)
	}
	res, err = grepTool(c.Ctx(), grepArg{Pattern: "^a", Path: "b.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 || res.Matches[0].Line != 1 || res.Matches[0].Text != "a\rb" {
		t.Fatalf("BOM line: got %+v", res.Matches)
	}
}

func TestLineCutsKeepWholeRunes(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "u.txt", "x"+strings.Repeat("é", 1500)+"\n")
	out, err := readTool(c.Ctx(), readArg{Path: "u.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "1: x" + strings.Repeat("é", 999) + "... (line truncated)\n(end of file, 1 lines)"; out != want {
		t.Fatalf("read: got %q", out)
	}
	res, err := grepTool(c.Ctx(), grepArg{Pattern: "x", Path: "u.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 || res.Matches[0].Text != "x"+strings.Repeat("é", 199)+"... (line truncated)" {
		t.Fatalf("grep: got %+v", res.Matches)
	}
}

func TestLongToolsStopOnCancel(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "a/b.txt", "needle\n")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("inner.txt")
	io.WriteString(w, "x")
	zw.Close()
	if _, err := writeBytesTool(c.Ctx(), writeBytesArg{Path: "a.zip", Data: buf.Bytes()}); err != nil {
		t.Fatal(err)
	}
	stop := errors.New("stopped")
	ctx, cancel := context.WithCancelCause(c.Ctx())
	cancel(stop)
	if _, err := grepTool(ctx, grepArg{Pattern: "needle"}); !errors.Is(err, stop) {
		t.Fatalf("grep: %v", err)
	}
	if _, err := grepTool(ctx, grepArg{Pattern: "needle", Path: "a/b.txt"}); !errors.Is(err, stop) {
		t.Fatalf("grep file: %v", err)
	}
	if _, err := readTool(ctx, readArg{Path: "a/b.txt"}); !errors.Is(err, stop) {
		t.Fatalf("read: %v", err)
	}
	if _, err := zipTool(ctx, zipArg{Paths: []string{"a"}, Dst: "out.zip"}); !errors.Is(err, stop) {
		t.Fatalf("zip: %v", err)
	}
	if _, err := unzipTool(ctx, unzipArg{Src: "a.zip", Dst: "out"}); !errors.Is(err, stop) {
		t.Fatalf("unzip: %v", err)
	}
}

func TestUnzipWithoutFilesListsAnEmptyArray(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.Create("dir/")
	zw.Close()
	if _, err := writeBytesTool(c.Ctx(), writeBytesArg{Path: "d.zip", Data: buf.Bytes()}); err != nil {
		t.Fatal(err)
	}
	res, err := unzipTool(c.Ctx(), unzipArg{Src: "d.zip"})
	if err != nil {
		t.Fatal(err)
	}
	if out, _ := json.Marshal(res); !strings.Contains(string(out), `"extracted":[]`) {
		t.Fatalf("got %s", out)
	}
}

func TestUnzipRemovesTheCutEntry(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{UnzipLimit: 10}})
	fs, ctx := c.FS(), c.Ctx()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("first.txt")
	io.WriteString(w, "12345")
	w, _ = zw.Create("second.txt")
	io.WriteString(w, "1234567890")
	zw.Close()
	if _, err := writeBytesTool(ctx, writeBytesArg{Path: "a.zip", Data: buf.Bytes()}); err != nil {
		t.Fatal(err)
	}
	if _, err := unzipTool(ctx, unzipArg{Src: "a.zip", Dst: "out"}); err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("err = %v", err)
	}
	if _, err := fs.Stat(context.Background(), "out/first.txt"); err != nil {
		t.Fatalf("the complete entry: %v", err)
	}
	if _, err := fs.Stat(context.Background(), "out/second.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("the cut entry: %v", err)
	}
}

func TestUnzipRemovesTheEntryCutByCancel(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("big.txt")
	io.WriteString(w, strings.Repeat("x", 1<<20))
	zw.Close()
	f, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("stopped")
	cancel(cause)
	if _, err := unzipEntry(ctx, fs, f.File[0], "out/big.txt", -1); !errors.Is(err, cause) {
		t.Fatalf("err = %v", err)
	}
	if _, err := fs.Stat(context.Background(), "out/big.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("the cut entry: %v", err)
	}
}

func TestZipSkipsOverlappingArgs(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "src/main.go", "package main")
	write(t, fs, "src/b.go", "package main")
	if _, err := zipTool(c.Ctx(), zipArg{Paths: []string{"src", "src/main.go", "src/b.go", "src"}, Dst: "out.zip"}); err != nil {
		t.Fatal(err)
	}
	if names := zipNames(t, fs, "out.zip"); len(names) != 2 {
		t.Fatalf("got %v", names)
	}
}

func TestWalkStopsAtTheFirstStop(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	write(t, fs, "a/x.txt", "x")
	write(t, fs, "a/y.txt", "y")
	write(t, fs, "b/z.txt", "z")
	calls := 0
	cont, err := walkFiles(context.Background(), fs, ".", false, func(api.FSEntry) (bool, error) {
		calls++
		return false, nil
	})
	if err != nil || cont || calls != 1 {
		t.Fatalf("cont %v, err %v, calls %d", cont, err, calls)
	}
}

func TestFailedWriteBytesKeepsTheOriginal(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{MaxBytes: 20}})
	fs := c.FS()
	write(t, fs, "a.bin", "0123456789")
	if _, err := writeBytesTool(c.Ctx(), writeBytesArg{Path: "a.bin", Data: make([]byte, 15)}); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("got %v", err)
	}
	rc, err := fs.ReadAll(context.Background(), "a.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if b, _ := io.ReadAll(rc); string(b) != "0123456789" {
		t.Fatalf("got %q", b)
	}
}

func TestReadFromScriptIsTheText(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	content := `{"cards":[1,2]}` + "\n" + strings.Repeat("y", 3000) + "\r\nlast"
	write(t, c.FS(), "a.txt", content)
	tool, err := tools.NewTool("read", readDesc(common.TierDeveloper), readTool, false)
	if err != nil {
		t.Fatal(err)
	}
	call := tool.JS(c.Ctx())
	out, err := call([]any{`{"path":"a.txt"}`})
	if err != nil || out != content {
		t.Fatalf("a script read %q, %v; want the file as it is", out, err)
	}
	out, err = call([]any{`{"path":"a.txt","offset":2,"limit":1}`})
	if want := strings.Repeat("y", 3000) + "\r\n"; err != nil || out != want {
		t.Fatalf("a script read of line 2 gave %q, %v; want the whole line with its ending", out, err)
	}
	if _, err := call([]any{`{"path":"a.txt","offset":9}`}); err == nil {
		t.Fatal("a script read past the end succeeded")
	}
}

func TestToolsTakeRootedAndFileURLPaths(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs, store := c.FS(), c.Store()
	write(t, fs, "sub/one.txt", "needle")
	write(t, fs, "sub/notes.txt", "needle")
	putFile(t, store, api.OriginAgent, "out.txt", "needle")
	for _, root := range []string{"sub", "/sub", "file:///sub", "file:///sub/"} {
		res, err := globTool(c.Ctx(), globArg{Pattern: "o*", Path: root})
		if err != nil || !slices.Equal(res.Paths, []string{"sub/one.txt"}) {
			t.Errorf("glob o* in %q: %+v, %v", root, res, err)
		}
		gres, err := grepTool(c.Ctx(), grepArg{Pattern: "needle", Path: root, Glob: "o*"})
		if err != nil || len(gres.Matches) != 1 || gres.Matches[0].Path != "sub/one.txt" {
			t.Errorf("grep o* in %q: %+v, %v", root, gres, err)
		}
	}
	for _, root := range []string{"agent_uploads", "/agent_uploads", "file:///agent_uploads", "file:///agent_uploads/"} {
		res, err := globTool(c.Ctx(), globArg{Pattern: "o*", Path: root})
		if err != nil || !slices.Equal(res.Paths, []string{"agent_uploads/out.txt"}) {
			t.Errorf("glob o* in %q: %+v, %v", root, res, err)
		}
		gres, err := grepTool(c.Ctx(), grepArg{Pattern: "needle", Path: root, Glob: "o*"})
		if err != nil || len(gres.Matches) != 1 || gres.Matches[0].Path != "agent_uploads/out.txt" {
			t.Errorf("grep o* in %q: %+v, %v", root, gres, err)
		}
	}
	for _, root := range []string{"/", "file:///"} {
		res, err := globTool(c.Ctx(), globArg{Pattern: "sub/o*", Path: root})
		if err != nil || !slices.Equal(res.Paths, []string{"sub/one.txt"}) {
			t.Errorf("glob sub/o* in %q: %+v, %v", root, res, err)
		}
	}
	for _, dst := range []string{"/z.zip", "file:///z.zip"} {
		e, err := zipTool(c.Ctx(), zipArg{Paths: []string{"."}, Dst: dst})
		if err != nil || e.Path != "z.zip" {
			t.Fatalf("zip to %q: %+v, %v", dst, e, err)
		}
		if names := zipNames(t, fs, "z.zip"); !slices.Equal(names, []string{"sub/notes.txt", "sub/one.txt"}) {
			t.Fatalf("zip to %q holds %v", dst, names)
		}
	}
	if _, err := zipTool(c.Ctx(), zipArg{Paths: []string{"sub"}, Dst: "file:///agent_uploads/z.zip"}); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("zip into agent_uploads by file URL: %v", err)
	}
	res, err := unzipTool(c.Ctx(), unzipArg{Src: "file:///z.zip", Dst: "file:///out"})
	if err != nil || res.Count != 2 {
		t.Fatalf("unzip: %+v, %v", res, err)
	}
	rc, err := fs.ReadAll(c.Ctx(), "out/sub/one.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "needle" {
		t.Fatalf("unzipped %q", got)
	}
	entries, err := fs.ReadDir(c.Ctx(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name, "file:") {
			t.Fatalf("the workspace root holds %q", e.Name)
		}
	}
}
