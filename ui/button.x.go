// Managed by GoX v0.3.3

//line button.gox:1
package ui

import (
	"context"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	neui "neui.dev"
	"neui.dev/api"
)

type buttonPatch struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text,omitempty" minLength:"1" description:"new label; omit to keep the current one"`
	Enabled string `json:"enabled,omitempty" enum:"yes,no" description:"yes lets the user click it, no disables it; omit to keep the current state"`
}

type buttonParams struct {
	_ struct{} `additionalProperties:"false"`
	Size string `json:"size" enum:"sm,md,lg" default:"md" description:"button size"`
	Style string `json:"style" enum:"primary,secondary,success,danger" default:"secondary" description:"visual weight: primary for the main action, danger for destructive ones"`
	Text string `json:"text" required:"true" minLength:"1" description:"label shown on the button"`
	Enabled string `json:"enabled" enum:"yes,no" default:"yes" description:"no shows the button disabled"`
	Value string `json:"value,omitempty" description:"data the click submits; empty when omitted"`
	Agent string `json:"agent,omitempty" description:"when set, the agent can click this button on the page as the user would; tells the agent what the click does"`
}

// NewButton builds a button widget for Library.Control: a click submits the
// button's value param to the script waiting in ui.draw(). With agent set, the
// button is also an Input of the enclosing space, so the agent can click it.
func NewButton(p buttonParams, submit func(ctx context.Context, value string) bool) api.Widget[buttonPatch] {
	if p.Size == "" {
		p.Size = "md"
	}
	if p.Style == "" {
		p.Style = "secondary"
	}
	if p.Enabled == "" {
		p.Enabled = "yes"
	}
	return button{
		state: doors.NewSource(p),
		submit: submit,
	}
}

type button struct {
	state doors.Source[buttonParams]
	submit func(ctx context.Context, value string) bool
}

func (b button) Update(ctx context.Context, patch buttonPatch) {
	b.state.Mutate(ctx, func(p buttonParams) buttonParams {
		if patch.Text != "" {
			p.Text = patch.Text
		}
		if patch.Enabled != "" {
			p.Enabled = patch.Enabled
		}
		return p
	})
}

//line button.gox:64
func (b button) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line button.gox:65
		__e = __c.Any(b.state.Bind(b.render)); if __e != nil { return }
	return })
//line button.gox:66
}

func (p buttonParams) input() gox.Proxy {
	if p.Agent == "" {
		return gox.ProxyFunc(func(cur gox.Cursor, el gox.Elem) error {
			return el(cur)
		})
	}
	return neui.Input{Desc: p.Agent}
}

//line button.gox:77
func (b button) render(p buttonParams) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line button.gox:78
		__e = (p.input()).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line button.gox:78
			__e = (doors.AClick{
		Scope: neui.IslandScope(ctx),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			b.submit(ctx, p.Value)
			return false
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line button.gox:85
					__e = __c.Set("type", "button"); if __e != nil { return }
//line button.gox:86
					__e = __c.Set("class", "kisland-button kisland-button-" + p.Size + " kisland-button-" + p.Style); if __e != nil { return }
//line button.gox:87
					__e = __c.Set("disabled", p.Enabled == "no"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line button.gox:88
					__e = __c.Any(p.Text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line button.gox:90
}
