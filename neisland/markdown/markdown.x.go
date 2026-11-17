// Managed by GoX v0.4.0

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
	Text *string `json:"text,omitempty" minLength:"0" maxLength:"4000" description:"new text, markdown as the text param; omit to keep the current one"`
	Style string `json:"style,omitempty" enum:"plain,card,info,success,warning,danger" description:"new style, as the style param; omit to keep the current one"`
}

// Params is what a script passes to ui.markdown.
type Params struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text" maxLength:"4000" required:"true" minLength:"0" description:"markdown: # ## ### headings, the only way to make text larger, **bold**, *italic*, ~~strike~~, code in backquotes, - and 1. lists, - [ ] task lists, > quotes, fenced code blocks, | tables |, [links](https://…); a line break is kept; [name](file:///path) to a file in the workspace, path relative to its root, shows a download of it, ![alt](file:///path) shows the image; raw HTML is dropped; \"\" shows nothing, whatever the style"`
	Style string `json:"style,omitempty" enum:"plain,card,info,success,warning,danger" default:"plain" description:"plain for titles and short lines; card frames content such as a report or a file list; info, success, warning and danger are a card marked in that tone, for outcomes that must stand out; the tone is only a mark, so the text must still say the outcome in words"`
}

var toneLabels = map[string]string{"info": "Info:", "success": "Success:", "warning": "Warning:", "danger": "Error:"}

// New builds a markdown widget for Library.View, framed in
// a card when its style param is not plain, the card marked in a tone
// for info, success, warning and danger. An empty text shows nothing,
// whatever the style.
func New(ctx context.Context, p Params) (api.Widget[Patch], error) {
	if p.Style == "" {
		p.Style = "plain"
	}
	return widget{state: doors.NewSource(p)}, nil
}

type widget struct {
	state doors.Source[Params]
}

type textView struct {
	Text  string `json:"text" required:"true" description:"the text, as markdown"`
	Style string `json:"style" required:"true" description:"plain, card, or the card's tone"`
}

func (t widget) getText(ctx context.Context, _ struct{}) (textView, error) {
	p, ok := t.state.Read(ctx)
	if !ok {
		return textView{}, context.Canceled
	}
	return textView{Text: p.Text, Style: p.Style}, nil
}

func (t widget) Update(ctx context.Context, patch Patch) {
	t.state.Mutate(ctx, func(p Params) Params {
		if patch.Text != nil {
			p.Text = *patch.Text
		}
		if patch.Style != "" {
			p.Style = patch.Style
		}
		return p
	})
}

//line markdown.gox:70
func (t widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line markdown.gox:71
		__e = __c.Any(neui.Tool[struct{}, textView]{Name: getTextName, Desc: getTextDesc, Handler: t.getText}); if __e != nil { return }
//line markdown.gox:72
		__e = __c.Any(t.state.Bind(t.render)); if __e != nil { return }
	return })
//line markdown.gox:73
}

//line markdown.gox:75
func (t widget) render(p Params) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line markdown.gox:77
		if p.Text == "" {
		return nil
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line markdown.gox:82
			__e = __c.Set("class", func() any {
			switch p.Style {
			case "plain":
				return "neisland-text neisland-prose"
			case "card":
				return "neisland-text neisland-prose neisland-text-card"
			}
			return "neisland-text neisland-prose neisland-text-card neisland-tone-" + p.Style
		}()); if __e != nil { return }
//line markdown.gox:91
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
//line markdown.gox:100
			if label := toneLabels[p.Style]; label != "" {
				__e = __c.Init("span"); if __e != nil { return }
				{
//line markdown.gox:101
					__e = __c.Set("class", "neisland-tone-label"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line markdown.gox:101
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line markdown.gox:103
			__e = __c.Any(files.MarkdownBody(neui.IslandFS(ctx), markdown.Block(p.Text))); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line markdown.gox:105
}
