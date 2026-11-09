// Managed by GoX v0.3.4

//line text.gox:1
package ui

import (
	"context"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
)

type textPatch struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text,omitempty" minLength:"1" maxLength:"4000" description:"new text; omit to keep the current one"`
	Style string `json:"style,omitempty" enum:"plain,info,success,warning,danger" description:"new style; plain turns an alert back into text; omit to keep the current one"`
}

type textParams struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text" maxLength:"4000" required:"true" minLength:"1" description:"the text; line breaks are kept"`
	Size string `json:"size,omitempty" enum:"sm,md,lg,xl,2xl" default:"md" description:"text size; lg and above read as headings"`
	Style string `json:"style,omitempty" enum:"plain,info,success,warning,danger" default:"plain" description:"plain is ordinary text; any other style shows it as an alert box of that kind"`
}

// NewText builds a text widget for Library.View: plain text in a size
// preset, or an alert box when its style param is not plain.
func NewText(ctx context.Context, p textParams) (api.Widget[textPatch], error) {
	if p.Size == "" {
		p.Size = "md"
	}
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

//line text.gox:52
func (t text) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line text.gox:53
		__e = __c.Any(t.state.Bind(t.render)); if __e != nil { return }
	return })
//line text.gox:54
}

//line text.gox:56
func (t text) render(p textParams) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line text.gox:58
			__e = __c.Set("class", func() any {
			base := "kisland-text kisland-text-" + p.Size
			if p.Style == "plain" {
				return base
			}
			return base + " kisland-alert kisland-alert-" + p.Style
		}()); if __e != nil { return }
//line text.gox:65
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
//line text.gox:74
			__e = __c.Any(p.Text); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line text.gox:76
}
