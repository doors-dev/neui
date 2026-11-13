// Managed by GoX v0.3.6-0.20261113082228-02db5fed593a

//line markdown.gox:1
package markdown

import (
	"context"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/markdown"
	"neui.dev/neisland/internal/files"
)

// Patch is what a script passes to update a markdown view.
type Patch struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text,omitempty" minLength:"1" maxLength:"4000" description:"new text, markdown as the text param; omit to keep the current one"`
	Style string `json:"style,omitempty" enum:"plain,card,info,success,warning,danger" description:"new style, as the style param; omit to keep the current one"`
}

// Params is what a script passes to ui.markdown.
type Params struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text" maxLength:"4000" required:"true" minLength:"1" description:"markdown: # ## ### headings, the only way to make text larger, **bold**, *italic*, ~~strike~~, code in backquotes, - and 1. lists, - [ ] task lists, > quotes, fenced code blocks, | tables |, [links](https://…); a line break is kept; [name](file:///path) to a file in the workspace, path relative to its root, shows a download of it, ![alt](file:///path) shows the image; raw HTML is dropped"`
	Style string `json:"style,omitempty" enum:"plain,card,info,success,warning,danger" default:"plain" description:"plain for titles and short lines; card frames content such as a report or a file list; info, success, warning and danger are a card marked in that tone, for outcomes that must stand out; the tone is only a mark, so the text must still say the outcome in words"`
}

var toneLabels = map[string]string{"info": "Info:", "success": "Success:", "warning": "Warning:", "danger": "Error:"}

// New builds a markdown widget for Library.View, framed in
// a card when its style param is not plain, the card marked in a tone
// for info, success, warning and danger.
func New(ctx context.Context, p Params) (api.Widget[Patch], error) {
	if p.Style == "" {
		p.Style = "plain"
	}
	return widget{state: doors.NewSource(p)}, nil
}

type widget struct {
	state doors.Source[Params]
}

func (t widget) Update(ctx context.Context, patch Patch) {
	t.state.Mutate(ctx, func(p Params) Params {
		if patch.Text != "" {
			p.Text = patch.Text
		}
		if patch.Style != "" {
			p.Style = patch.Style
		}
		return p
	})
}

//line markdown.gox:56
func (t widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line markdown.gox:57
		__e = __c.Any(t.state.Bind(t.render)); if __e != nil { return }
	return })
//line markdown.gox:58
}

//line markdown.gox:60
func (t widget) render(p Params) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line markdown.gox:62
			__e = __c.Set("class", func() any {
			switch p.Style {
			case "plain":
				return "neisland-text neisland-prose"
			case "card":
				return "neisland-text neisland-prose neisland-text-card"
			}
			return "neisland-text neisland-prose neisland-text-card neisland-tone-" + p.Style
		}()); if __e != nil { return }
//line markdown.gox:71
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
//line markdown.gox:80
			if label := toneLabels[p.Style]; label != "" {
				__e = __c.Init("span"); if __e != nil { return }
				{
//line markdown.gox:81
					__e = __c.Set("class", "neisland-tone-label"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line markdown.gox:81
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line markdown.gox:83
			__e = __c.Any(files.MarkdownBody(neui.IslandFS(ctx), markdown.Block(p.Text))); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line markdown.gox:85
}
