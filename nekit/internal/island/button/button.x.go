// Managed by GoX v0.4.0

//line button.gox:1
package button

import (
	"context"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/nekit/internal/island/internal/pending"
	"neui.dev/api"
)

// Patch is what a script passes to update a button.
type Patch struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text,omitempty" minLength:"1" maxLength:"80" description:"new label; omit to keep the current one"`
	Disabled *bool `json:"disabled,omitempty" description:"true disables the button, false enables it; omit to keep the current state"`
}

// Params is what a script passes to ui.button.
type Params struct {
	_ struct{} `additionalProperties:"false"`
	Size string `json:"size,omitempty" enum:"sm,md,lg" default:"md"`
	Style string `json:"style,omitempty" enum:"primary,secondary,success,danger" default:"secondary" description:"visual weight: primary for the main action, danger for destructive ones"`
	Text string `json:"text" maxLength:"80" required:"true" minLength:"1" description:"label shown on the button"`
	Disabled bool `json:"disabled,omitempty" description:"true disables the button: clicks do nothing"`
	Value string `json:"value,omitempty" maxLength:"1000" description:"data the click submits; empty when omitted"`
}

// New builds a button widget for Library.Control: a click on the enabled
// button submits its value param to the script waiting in ui.draw(). It is
// also an Input described by its text, so the agent can click it.
func New(ctx context.Context, p Params, submit func(ctx context.Context, value string) bool) (api.Widget[Patch], error) {
	if p.Size == "" {
		p.Size = "md"
	}
	if p.Style == "" {
		p.Style = "secondary"
	}
	return widget{
		state: doors.NewSource(p),
		submit: submit,
	}, nil
}

type widget struct {
	state doors.Source[Params]
	submit func(ctx context.Context, value string) bool
}

func (b widget) Update(ctx context.Context, patch Patch) {
	b.state.Mutate(ctx, func(p Params) Params {
		if patch.Text != "" {
			p.Text = patch.Text
		}
		if patch.Disabled != nil {
			p.Disabled = *patch.Disabled
		}
		return p
	})
}


//line button.gox:64
func (b widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line button.gox:65
		__e = __c.Any(b.state.Bind(b.render)); if __e != nil { return }
	return })
//line button.gox:66
}

//line button.gox:68
func (b widget) render(p Params) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line button.gox:69
		__e = (neui.Input{Desc: p.Text}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line button.gox:69
			__e = (doors.AClick{
		Scope: neui.IslandScope(ctx),
		Indicator: pending.Indicator(ctx),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			if b.state.Get().Disabled {
				return false
			}
			b.submit(ctx, p.Value)
			return false
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line button.gox:80
					__e = __c.Set("type", "button"); if __e != nil { return }
//line button.gox:81
					__e = __c.Set("class", "neisland-button neisland-button-" + p.Size + " neisland-button-" + p.Style); if __e != nil { return }
//line button.gox:82
					__e = __c.Set("value", p.Value); if __e != nil { return }
//line button.gox:83
					__e = __c.Set("disabled", p.Disabled); if __e != nil { return }
//line button.gox:84
					__e = __c.Set("data-neui", "submit"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line button.gox:85
					__e = __c.Any(p.Text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line button.gox:87
}
