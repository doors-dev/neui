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

package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/doors-dev/gox"
	_ "golang.org/x/image/webp"
	"rsc.io/pdf"

	"neui.dev/api"
)

func memoryLimit(config api.ThreadConfig, model api.Model, factor float32) int64 {
	limit := config.ThreadMemoryLimit
	if limit > 0 && model.RequestBytes > 0 {
		unencodedRequestBytes := int64(model.RequestBytes) * 3 / 4
		afterHeadroom := unencodedRequestBytes - api.ReplayHeadroom
		modelLimit := max(afterHeadroom, 1)
		limit = min(limit, modelLimit)
	}
	if limit > 0 && factor < 1 {
		tightened := float32(limit) * factor
		limit = max(int64(tightened), 1)
	}
	return limit
}

const (
	evictionKeptParts   = 3
	evictionBudgetParts = 5
)

func evictionTarget[T int | int64](budget T) T {
	kept := budget * evictionKeptParts
	return kept / evictionBudgetParts
}

const (
	bytesPerToken  = 4
	unknownContext = 200_000
)

func RoundItemBudget(config api.ThreadConfig, model api.Model) int {
	tokens := model.Context
	if tokens == 0 {
		tokens = unknownContext
	}
	contextBytes := float32(tokens) * bytesPerToken
	return int(contextBytes * config.ReplayItemShare)
}

func OmittedNote(size int) string {
	human := fmt.Sprintf("%d bytes", size)
	if size >= 1<<20 {
		human = fmt.Sprintf("%.1f MB", float64(size)/(1<<20))
	} else if size >= 1<<10 {
		human = fmt.Sprintf("%.1f KB", float64(size)/(1<<10))
	}
	return fmt.Sprintf("Result omitted: it was %s, more than a turn's tool results can hold. Ask for less, or process it in a script.", human)
}

func capText(c api.ContentToolResult, cap int) api.ContentToolResult {
	texts := make([]api.ContentText, len(c.Content))
	indices := make([]int, 0, len(c.Content))
	total := 0
	for i, block := range c.Content {
		text, ok := block.(api.ContentText)
		if !ok {
			continue
		}
		texts[i] = text
		indices = append(indices, i)
		total += len(text.Text)
	}
	if total <= cap {
		return c
	}
	slices.SortFunc(indices, func(a, b int) int { return len(texts[a].Text) - len(texts[b].Text) })
	shares := make([]int, len(c.Content))
	remaining, left := cap, len(indices)
	for _, i := range indices {
		share := remaining / left
		if len(texts[i].Text) > share {
			shares[i] = share
			continue
		}
		shares[i] = len(texts[i].Text)
		remaining -= shares[i]
		left--
	}
	content := slices.Clone(c.Content)
	for _, i := range indices {
		content[i] = trimText(texts[i], shares[i])
	}
	return api.ContentToolResult{
		ID:      c.ID,
		Content: content,
		Error:   c.Error,
	}
}

func trimText(text api.ContentText, share int) api.ContentText {
	if len(text.Text) <= share {
		return text
	}
	reach := min(share/8, 1024)
	head := share / 2
	for j := head - 1; j >= max(head-reach, 0); j-- {
		if text.Text[j] == '\n' {
			head = j + 1
			break
		}
	}
	for step := 0; step <= 3; step++ {
		if head-step == 0 || utf8.RuneStart(text.Text[head-step]) {
			head -= step
			break
		}
	}
	tail := len(text.Text) - (share - share/2)
	for j := tail - 1; j < min(tail+reach, len(text.Text)); j++ {
		if text.Text[j] == '\n' {
			tail = j + 1
			break
		}
	}
	for step := 0; step <= 3; step++ {
		if tail+step == len(text.Text) || utf8.RuneStart(text.Text[tail+step]) {
			tail += step
			break
		}
	}
	trimmed := text.Text[:head] + fmt.Sprintf("<truncated>%d bytes omitted</truncated>", tail-head) + text.Text[tail:]
	if len(trimmed) >= len(text.Text) {
		return text
	}
	return api.ContentText{Text: trimmed, Meta: text.Meta}
}

func recordBody(rec api.Record) string {
	switch data := rec.Data.(type) {
	case string:
		return html.EscapeString(data)
	case json.RawMessage:
		if !bytes.ContainsAny(data, "<>&") {
			return string(data)
		}
		var esc bytes.Buffer
		json.HTMLEscape(&esc, data)
		return esc.String()
	default:
		raw, err := json.Marshal(rec.Data)
		if err != nil {
			return html.EscapeString("OUTPUT ENCODING ERROR: " + err.Error())
		}
		return string(raw)
	}
}

func recordContent(rec api.Record, cap int) api.Content {
	body := trimText(api.ContentText{Text: recordBody(rec)}, cap).Text
	return api.ContentText{Text: element(rec.Kind, rec.Attrs, body)}
}

func noteContent(kind string, text string) api.Content {
	return api.ContentText{Text: element(kind, nil, strings.ReplaceAll(text, "</"+kind, "&lt;/"+kind))}
}

func element(kind string, attrs map[string]any, body string) string {
	el := gox.Elem(func(cur gox.Cursor) error {
		cur.Init(kind)
		for _, key := range slices.Sorted(maps.Keys(attrs)) {
			cur.Set(key, attrs[key])
		}
		cur.Submit()
		cur.Raw(body)
		cur.Close()
		return nil
	})
	b := strings.Builder{}
	el.Render(context.Background(), &b)
	return b.String()
}

func weigh(content ...api.Content) int64 {
	weight := int64(0)
	for _, block := range content {
		switch b := block.(type) {
		case api.ContentText:
			weight += int64(len(b.Text))
		case api.ContentImage:
			weight += int64(len(b.Data))
		case api.ContentDocument:
			weight += int64(len(b.Data))
		case api.ContentToolResult:
			for _, nested := range b.Content {
				switch n := nested.(type) {
				case api.ContentText:
					weight += int64(len(n.Text))
				case api.ContentImage:
					weight += int64(len(n.Data))
				case api.ContentDocument:
					weight += int64(len(n.Data))
				}
			}
		case api.ContentToolUse:
			weight += int64(len(b.Meta))
			if !bytes.Equal(b.Input, api.DiscardedArguments) {
				weight += int64(len(b.Input))
			}
		case api.ContentReasoning:
			for _, part := range b.Summary {
				weight += int64(len(part))
			}
			for _, part := range b.Thoughts {
				weight += int64(len(part))
			}
			weight += int64(len(b.Meta))
		case api.ContentOpaque:
			weight += int64(len(b.Data))
			for _, delta := range b.Deltas {
				weight += int64(len(delta))
			}
		}
	}
	return weight
}

func countImages(content ...api.Content) int {
	images := 0
	for _, block := range content {
		switch b := block.(type) {
		case api.ContentImage:
			images++
		case api.ContentDocument:
			images += pdfPages(b)
		case api.ContentToolResult:
			for _, inner := range b.Content {
				switch n := inner.(type) {
				case api.ContentImage:
					images++
				case api.ContentDocument:
					images += pdfPages(n)
				}
			}
		}
	}
	return images
}

const maxPDFPages = 600

func pdfPages(c api.ContentDocument) (pages int) {
	if api.DocumentMime(c.Mime) != api.DocumentPDF {
		return 0
	}
	defer func() {
		if recover() != nil {
			pages = 0
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(c.Data), int64(len(c.Data)))
	if err != nil {
		return 0
	}
	if n := r.NumPage(); n > 0 {
		return min(n, maxPDFPages)
	}
	return 0
}

const evictedMedia = "removed to fit this request; do not describe its contents from memory — read it again with a tool, or ask the user to attach it again"

const rejectedMedia = "the provider rejected the request carrying it; do not describe its contents from memory — read it again with a tool, or ask the user to attach it again"

func omitAllMedia(content []api.Content, reason string) []api.Content {
	changed := false
	out := make([]api.Content, 0, len(content))
	for _, block := range content {
		switch b := block.(type) {
		case api.ContentImage:
			changed = true
			out = append(out, noImage(b, reason))
		case api.ContentDocument:
			changed = true
			out = append(out, noDocument(b, reason))
		case api.ContentToolResult:
			inner := make(api.UserContents, 0, len(b.Content))
			for _, nested := range b.Content {
				switch n := nested.(type) {
				case api.ContentImage:
					changed = true
					inner = append(inner, noImage(n, reason))
				case api.ContentDocument:
					changed = true
					inner = append(inner, noDocument(n, reason))
				default:
					inner = append(inner, nested)
				}
			}
			out = append(out, api.ContentToolResult{ID: b.ID, Content: inner, Error: b.Error})
		default:
			out = append(out, block)
		}
	}
	if !changed {
		return content
	}
	return out
}

func omitOversizedMedia(content []api.Content, m api.Model, documentCap int64) []api.Content {
	changed := false
	out := make([]api.Content, 0, len(content))
	for _, block := range content {
		switch b := block.(type) {
		case api.ContentImage:
			if reason := imageReason(b, m); reason != "" {
				changed = true
				out = append(out, noImage(b, reason))
				continue
			}
			out = append(out, block)
		case api.ContentDocument:
			if documentCap > 0 && int64(len(b.Data)) > documentCap {
				changed = true
				out = append(out, noDocument(b, fmt.Sprintf("%d bytes is over the %d this model reads in one request", len(b.Data), documentCap)))
				continue
			}
			out = append(out, block)
		case api.ContentToolResult:
			inner := make(api.UserContents, 0, len(b.Content))
			for _, nested := range b.Content {
				switch n := nested.(type) {
				case api.ContentImage:
					if reason := imageReason(n, m); reason != "" {
						changed = true
						inner = append(inner, noImage(n, reason))
						continue
					}
					inner = append(inner, nested)
				case api.ContentDocument:
					if documentCap > 0 && int64(len(n.Data)) > documentCap {
						changed = true
						inner = append(inner, noDocument(n, fmt.Sprintf("%d bytes is over the %d this model reads in one request", len(n.Data), documentCap)))
						continue
					}
					inner = append(inner, nested)
				default:
					inner = append(inner, nested)
				}
			}
			out = append(out, api.ContentToolResult{ID: b.ID, Content: inner, Error: b.Error})
		default:
			out = append(out, block)
		}
	}
	if !changed {
		return content
	}
	return out
}

func imageReason(c api.ContentImage, m api.Model) string {
	if m.ImageBytes > 0 && len(c.Data) > m.ImageBytes {
		return fmt.Sprintf("%d bytes is over the %d this model takes in one image", len(c.Data), m.ImageBytes)
	}
	if m.ImagePixels <= 0 {
		return ""
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(c.Data))
	if err != nil {
		return fmt.Sprintf("%s did not decode: %v", c.Mime, err)
	}
	if config.Width*config.Height > m.ImagePixels {
		return fmt.Sprintf("%dx%d is over the %d pixels this model reads in one image", config.Width, config.Height, m.ImagePixels)
	}
	return ""
}

func noImage(c api.ContentImage, reason string) api.ContentText {
	return api.ContentText{Text: fmt.Sprintf("<noimage mime=%q bytes=%d reason=%q/>", c.Mime, len(c.Data), reason)}
}

func noDocument(c api.ContentDocument, reason string) api.ContentText {
	return api.ContentText{Text: fmt.Sprintf("<nodocument name=%q mime=%q bytes=%d reason=%q/>", c.Name, c.Mime, len(c.Data), reason)}
}
