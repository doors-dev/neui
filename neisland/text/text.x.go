// Managed by GoX v0.3.4

//line text.gox:1
package text

import (
	"context"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/markdown"
	"neui.dev/neisland/internal/files"
)

type patch struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text,omitempty" minLength:"1" maxLength:"4000" description:"new text, markdown as the text param; omit to keep the current one"`
	Style string `json:"style,omitempty" enum:"plain,card,info,success,warning,danger" description:"new style; plain drops the box; omit to keep the current one"`
}

type params struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text" maxLength:"4000" required:"true" minLength:"1" description:"markdown: # ## ### headings, the only way to make text larger, **bold**, *italic*, ~~strike~~, code in backquotes, - and 1. lists, - [ ] task lists, > quotes, fenced code blocks, | tables |, [links](https://…); a line break is kept; [name](file:///path) to a file in the workspace, path relative to its root, shows a download of it, ![alt](file:///path) shows the image; raw HTML is dropped"`
	Style string `json:"style,omitempty" enum:"plain,card,info,success,warning,danger" default:"plain" description:"plain for titles and short lines; card frames content such as a report or a file list; info, success, warning and danger are a card marked in that tone, for outcomes that must stand out; the tone is only a mark, so the text must still say the outcome in words"`
}

var toneLabels = map[string]string{"info": "Info:", "success": "Success:", "warning": "Warning:", "danger": "Error:"}

// New builds a text widget for Library.View: markdown, framed in
// a card when its style param is not plain, the card marked in a tone
// for info, success, warning and danger.
func New(ctx context.Context, p params) (api.Widget[patch], error) {
	if p.Style == "" {
		p.Style = "plain"
	}
	return widget{state: doors.NewSource(p)}, nil
}

type widget struct {
	state doors.Source[params]
}

func (t widget) Update(ctx context.Context, patch patch) {
	t.state.Mutate(ctx, func(p params) params {
		if patch.Text != "" {
			p.Text = patch.Text
		}
		if patch.Style != "" {
			p.Style = patch.Style
		}
		return p
	})
}

//line text.gox:54
func (t widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line text.gox:55
		__e = __c.Any(t.state.Bind(t.render)); if __e != nil { return }
	return })
//line text.gox:56
}

//line text.gox:58
func (t widget) render(p params) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line text.gox:60
			__e = __c.Set("class", func() any {
			switch p.Style {
			case "plain":
				return "neisland-text neisland-prose"
			case "card":
				return "neisland-text neisland-prose neisland-text-card"
			}
			return "neisland-text neisland-prose neisland-text-card neisland-tone-" + p.Style
		}()); if __e != nil { return }
//line text.gox:69
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
//line text.gox:78
			if label := toneLabels[p.Style]; label != "" {
				__e = __c.Init("span"); if __e != nil { return }
				{
//line text.gox:79
					__e = __c.Set("class", "neisland-tone-label"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line text.gox:79
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line text.gox:81
			__e = __c.Any(files.MarkdownBody(neui.IslandFS(ctx), markdown.Block(p.Text))); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line text.gox:83
}
