package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/media"
)

type blocks []api.UserContent

func (b blocks) Contents() []api.UserContent {
	return b
}

func (b blocks) MarshalJSON() ([]byte, error) {
	return json.Marshal(api.UserContents(b))
}

var _ api.MultiContainer = blocks{}

func result(ctx context.Context, res *mcp.CallToolResult, structured bool) any {
	if structured && res.StructuredContent != nil {
		return res.StructuredContent
	}
	var model api.Model
	if thread, ok := ctx.Value(common.KeyController).(common.Controller); ok {
		model = thread.Model()
	}
	out := make(blocks, 0, len(res.Content))
	for _, c := range res.Content {
		switch c := c.(type) {
		case *mcp.TextContent:
			out = append(out, api.ContentText{Text: c.Text})
		case *mcp.ImageContent:
			if mimeType := api.ImageMime(c.MIMEType); mimeType.IsValid() {
				if shaped, err := media.Shape(c.Data, mimeType, model); err == nil {
					out = append(out, api.ContentImage{Data: shaped.Data, Mime: string(shaped.Mime)})
					continue
				}
			}
			out = append(out, api.ContentText{Text: fmt.Sprintf("<omitted type=\"image\" mime=%q size=%d/>", c.MIMEType, len(c.Data))})
		case *mcp.AudioContent:
			out = append(out, api.ContentText{Text: fmt.Sprintf("<omitted type=\"audio\" mime=%q size=%d/>", c.MIMEType, len(c.Data))})
		case *mcp.ResourceLink:
			var b strings.Builder
			fmt.Fprintf(&b, "<resource uri=%q", c.URI)
			if c.Title != "" {
				fmt.Fprintf(&b, " title=%q", c.Title)
			} else if c.Name != "" {
				fmt.Fprintf(&b, " name=%q", c.Name)
			}
			if c.MIMEType != "" {
				fmt.Fprintf(&b, " mime=%q", c.MIMEType)
			}
			if c.Description != "" {
				fmt.Fprintf(&b, ">%s</resource>", c.Description)
			} else {
				b.WriteString("/>")
			}
			out = append(out, api.ContentText{Text: b.String()})
		case *mcp.EmbeddedResource:
			if c.Resource == nil {
				continue
			}
			if c.Resource.Text != "" {
				out = append(out, api.ContentText{Text: fmt.Sprintf("<resource uri=%q mime=%q>\n%s\n</resource>", c.Resource.URI, c.Resource.MIMEType, c.Resource.Text)})
			} else if api.DocumentMime(c.Resource.MIMEType).IsValid() {
				out = append(out, api.ContentDocument{Data: c.Resource.Blob, Mime: c.Resource.MIMEType, Name: c.Resource.URI})
			} else {
				out = append(out, api.ContentText{Text: fmt.Sprintf("<resource uri=%q mime=%q size=%d omitted/>", c.Resource.URI, c.Resource.MIMEType, len(c.Resource.Blob))})
			}
		}
	}
	return out
}

func inputRequired(requests mcp.InputRequestMap) error {
	kinds := make([]string, 0, len(requests))
	for _, r := range requests {
		switch r.(type) {
		case *mcp.ElicitParams:
			kinds = append(kinds, "a question for the user")
		case *mcp.CreateMessageParams, *mcp.CreateMessageWithToolsParams:
			kinds = append(kinds, "an LLM completion")
		case *mcp.ListRootsParams:
			kinds = append(kinds, "the client's root directories")
		default:
			kinds = append(kinds, "unknown input")
		}
	}
	slices.Sort(kinds)
	kinds = slices.Compact(kinds)
	if len(kinds) == 0 {
		return errors.New("the server postponed this call and asked to retry it later")
	}
	return fmt.Errorf("this tool needs interactive input this client does not provide (%s), so the call cannot be completed here; do not retry it, tell the user", strings.Join(kinds, ", "))
}

func resultError(res *mcp.CallToolResult) error {
	parts := make([]string, 0, len(res.Content))
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	if len(parts) == 0 {
		return errors.New("the tool reported an error without a message")
	}
	return errors.New(strings.Join(parts, "\n"))
}
