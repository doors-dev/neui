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

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

var _ api.Container = blocks{}

func result(ctx context.Context, res *mcp.CallToolResult, structured bool) any {
	if structured && res.StructuredContent != nil {
		return res.StructuredContent
	}
	var model api.Model
	documentLimit, found := 0, false
	if thread, ok := ctx.Value(common.KeyController).(common.Controller); ok {
		model = thread.Model()
		documentLimit, found = thread.FilesConfig().DocumentLimit, true
	}
	out := make(blocks, 0, len(res.Content))
	for _, c := range res.Content {
		switch c := c.(type) {
		case *mcp.TextContent:
			out = append(out, api.ContentText{Text: c.Text})
		case *mcp.ImageContent:
			if mimeType := api.ImageMime(http.DetectContentType(c.Data)); mimeType.IsValid() {
				if mimeType != api.ImageGIF || media.StillGIF(c.Data) == nil {
					if shaped, err := media.Shape(c.Data, mimeType, model); err == nil {
						out = append(out, api.ContentImage{Data: shaped.Data, Mime: string(shaped.Mime)})
						continue
					}
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
			} else if mimeType := api.DocumentMime(c.Resource.MIMEType); mimeType.IsValid() && found && (documentLimit < 0 || len(c.Resource.Blob) <= documentLimit) &&
				(mimeType != api.DocumentPDF || bytes.HasPrefix(c.Resource.Blob, []byte("%PDF-"))) &&
				(mimeType != api.DocumentDOCX && mimeType != api.DocumentXLSX && mimeType != api.DocumentPPTX || bytes.HasPrefix(c.Resource.Blob, []byte("PK"))) {
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
			kinds = append(kinds, "root directories")
		default:
			kinds = append(kinds, "unknown input")
		}
	}
	slices.Sort(kinds)
	kinds = slices.Compact(kinds)
	if len(kinds) == 0 {
		return errors.New("this tool's server is busy; retry later")
	}
	return fmt.Errorf("this tool asked for input the application cannot give (%s); do not retry it, tell the user", strings.Join(kinds, ", "))
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
