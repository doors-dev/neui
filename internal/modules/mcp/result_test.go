package mcp

import (
	"bytes"
	"image"
	"image/png"
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
	})
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
	})
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
