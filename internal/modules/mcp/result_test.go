package mcp

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"neui.dev/api"
	"neui.dev/internal/testkit"
)

func TestResultShapesImage(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4000, 3000))); err != nil {
		t.Fatal(err)
	}
	pixels := 1568 * 1568
	out := result(testkit.NewController(t, testkit.Setup{Model: api.Model{ImagePixels: pixels}}).Ctx(), &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.ImageContent{MIMEType: "image/png", Data: buf.Bytes()}},
	}, false)
	contents, ok := out.(blocks)
	if !ok {
		t.Fatalf("got %T, want blocks", out)
	}
	if len(contents) != 1 {
		t.Fatalf("got %d blocks, want 1", len(contents))
	}
	img, ok := contents[0].(api.ContentImage)
	if !ok {
		t.Fatalf("got %T, want api.ContentImage", contents[0])
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width*cfg.Height > pixels {
		t.Fatalf("%dx%d is %d pixels, over %d", cfg.Width, cfg.Height, cfg.Width*cfg.Height, pixels)
	}
}

func TestResultOmitsUnshapableImage(t *testing.T) {
	out := result(testkit.NewController(t, testkit.Setup{Model: api.Model{ImagePixels: 1568 * 1568}}).Ctx(), &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{MIMEType: "image/png", Data: []byte("not an image")},
			&mcp.ImageContent{MIMEType: "image/tiff", Data: []byte("nope")},
		},
	}, false)
	contents, ok := out.(blocks)
	if !ok {
		t.Fatalf("got %T, want blocks", out)
	}
	if len(contents) != 2 {
		t.Fatalf("got %d blocks, want 2", len(contents))
	}
	for i, c := range contents {
		text, ok := c.(api.ContentText)
		if !ok {
			t.Fatalf("block %d: got %T, want api.ContentText", i, c)
		}
		if !bytes.Contains([]byte(text.Text), []byte("<omitted type=\"image\"")) {
			t.Fatalf("block %d: %q", i, text.Text)
		}
	}
}

func TestResultUndeclaredStructuredIsBlocks(t *testing.T) {
	res := &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: "hello"}},
		StructuredContent: map[string]any{"a": 1},
	}
	contents, ok := result(context.Background(), res, false).(blocks)
	if !ok {
		t.Fatal("undeclared structured content returned instead of blocks")
	}
	if len(contents) != 1 {
		t.Fatalf("got %d blocks, want 1", len(contents))
	}
	if text, ok := contents[0].(api.ContentText); !ok || text.Text != "hello" {
		t.Fatalf("got %v, want the text block", contents[0])
	}
	if _, ok := result(context.Background(), res, true).(map[string]any); !ok {
		t.Fatal("declared structured content not returned")
	}
}

func TestResultOmitsAnimatedGIF(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White})
	var animated, still bytes.Buffer
	if err := gif.EncodeAll(&animated, &gif.GIF{Image: []*image.Paletted{frame, frame}, Delay: []int{10, 10}}); err != nil {
		t.Fatal(err)
	}
	if err := gif.EncodeAll(&still, &gif.GIF{Image: []*image.Paletted{frame}, Delay: []int{0}}); err != nil {
		t.Fatal(err)
	}
	out := result(testkit.NewController(t, testkit.Setup{}).Ctx(), &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{MIMEType: "image/gif", Data: animated.Bytes()},
			&mcp.ImageContent{MIMEType: "image/gif", Data: still.Bytes()},
			&mcp.ImageContent{MIMEType: "image/gif", Data: still.Bytes()[:still.Len()-1]},
		},
	}, false)
	contents := out.(blocks)
	if len(contents) != 3 {
		t.Fatalf("got %d blocks, want 3", len(contents))
	}
	if text, ok := contents[0].(api.ContentText); !ok || !strings.Contains(text.Text, "<omitted type=\"image\"") {
		t.Fatalf("animated GIF: got %#v", contents[0])
	}
	if img, ok := contents[1].(api.ContentImage); !ok || img.Mime != string(api.ImageGIF) {
		t.Fatalf("still GIF: got %#v", contents[1])
	}
	if _, ok := contents[2].(api.ContentText); !ok {
		t.Fatalf("truncated GIF: got %#v", contents[2])
	}
}

func TestResultImageMimeFromBytes(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	out := result(testkit.NewController(t, testkit.Setup{}).Ctx(), &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{MIMEType: "image/png", Data: buf.Bytes()},
			&mcp.ImageContent{MIMEType: "image/png", Data: []byte("GIF89a but not really")},
		},
	}, false)
	contents := out.(blocks)
	if img, ok := contents[0].(api.ContentImage); !ok || img.Mime != string(api.ImageJPEG) {
		t.Fatalf("got %#v, want image/jpeg", contents[0])
	}
	if _, ok := contents[1].(api.ContentText); !ok {
		t.Fatalf("got %#v, want omitted note", contents[1])
	}
}

func TestResultEmbeddedDocumentChecks(t *testing.T) {
	ctx := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{DocumentLimit: 16}}).Ctx()
	res := &mcp.CallToolResult{Content: []mcp.Content{
		&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "ok.pdf", MIMEType: "application/pdf", Blob: []byte("%PDF-1.7 small")}},
		&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "big.pdf", MIMEType: "application/pdf", Blob: []byte("%PDF-1.7 over the limit")}},
		&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "fake.pdf", MIMEType: "application/pdf", Blob: []byte("<html>")}},
		&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "fake.docx", MIMEType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", Blob: []byte("<html>")}},
	}}
	out := result(ctx, res, false).(blocks)
	if _, ok := out[0].(api.ContentDocument); !ok {
		t.Fatalf("block 0: got %T", out[0])
	}
	for i, c := range out[1:] {
		if text, ok := c.(api.ContentText); !ok || !strings.Contains(text.Text, "omitted") {
			t.Fatalf("block %d: got %#v", i+1, c)
		}
	}
	out = result(context.Background(), res, false).(blocks)
	if text, ok := out[0].(api.ContentText); !ok || !strings.Contains(text.Text, "omitted") {
		t.Fatalf("no controller: got %#v", out[0])
	}
}
