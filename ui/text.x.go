// Managed by GoX v0.3.4

//line text.gox:1
package ui

import (
	"context"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/markdown"
)

type textPatch struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text,omitempty" minLength:"1" maxLength:"4000" description:"new text, markdown as the text param; omit to keep the current one"`
	Style string `json:"style,omitempty" enum:"plain,card,info,success,warning,danger" description:"new style; plain drops the box; omit to keep the current one"`
}

type textParams struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text" maxLength:"4000" required:"true" minLength:"1" description:"markdown: # ## ### headings, the only way to make text larger, **bold**, *italic*, ~~strike~~, code in backquotes, - and 1. lists, - [ ] task lists, > quotes, fenced code blocks, | tables |, [links](https://…); a line break is kept; [name](file:///path) to a file in the workspace, path relative to its root, shows a download of it, ![alt](file:///path) shows the image; raw HTML is dropped"`
	Style string `json:"style,omitempty" enum:"plain,card,info,success,warning,danger" default:"plain" description:"plain for titles and short lines; card frames content such as a report or a file list; info, success, warning and danger are a card marked in that tone, for outcomes that must stand out; the tone is only a mark, so the text must still say the outcome in words"`
}

var textToneLabels = map[string]string{"info": "Info:", "success": "Success:", "warning": "Warning:", "danger": "Error:"}

// NewText builds a text widget for Library.View: markdown, framed in
// a card when its style param is not plain, the card marked in a tone
// for info, success, warning and danger.
func NewText(ctx context.Context, p textParams) (api.Widget[textPatch], error) {
	if p.Style == "" {
		p.Style = "plain"
	}
	return text{state: doors.NewSource(p)}, nil
}

type text struct {
	state doors.Source[textParams]
}

func (t text) Update(ctx context.Context, patch textPatch) {
	t.state.Mutate(ctx, func(p textParams) textParams {
		if patch.Text != "" {
			p.Text = patch.Text
		}
		if patch.Style != "" {
			p.Style = patch.Style
		}
		return p
	})
}

//line text.gox:53
func (t text) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line text.gox:54
		__e = __c.Any(t.state.Bind(t.render)); if __e != nil { return }
	return })
//line text.gox:55
}

//line text.gox:57
func (t text) render(p textParams) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line text.gox:59
			__e = __c.Set("class", func() any {
			switch p.Style {
			case "plain":
				return "kisland-text kisland-prose"
			case "card":
				return "kisland-text kisland-prose kisland-text-card"
			}
			return "kisland-text kisland-prose kisland-text-card kisland-tone-" + p.Style
		}()); if __e != nil { return }
//line text.gox:68
			__e = __c.Set("role", func() any {
			switch p.Style {
			case "warning", "danger":
				return "alert"
			case "info", "success":
				return "status"
			}
			return nil
		}()); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line text.gox:77
			if label := textToneLabels[p.Style]; label != "" {
				__e = __c.Init("span"); if __e != nil { return }
				{
//line text.gox:78
					__e = __c.Set("class", "kisland-tone-label"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line text.gox:78
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line text.gox:80
			__e = __c.Any(markdownBody(neui.IslandFS(ctx), markdown.Block(p.Text))); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line text.gox:82
}
