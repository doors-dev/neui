package mcp

import (
	"bytes"
	"context"
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
