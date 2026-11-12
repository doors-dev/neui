// Managed by GoX v0.3.4

//line button.gox:1
package button

import (
	"context"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	neui "neui.dev"
	"neui.dev/api"
)

type patch struct {
	_ struct{} `additionalProperties:"false"`
	Text string `json:"text,omitempty" minLength:"1" maxLength:"80" description:"new label; omit to keep the current one"`
	Disabled *bool `json:"disabled,omitempty" description:"true disables the button, false enables it; omit to keep the current state"`
}

type params struct {
	_ struct{} `additionalProperties:"false"`
	Size string `json:"size,omitempty" enum:"sm,md,lg" default:"md" description:"button size"`
	Style string `json:"style,omitempty" enum:"primary,secondary,success,danger" default:"secondary" description:"visual weight: primary for the main action, danger for destructive ones"`
	Text string `json:"text" maxLength:"80" required:"true" minLength:"1" description:"label shown on the button"`
	Disabled bool `json:"disabled,omitempty" description:"true shows the button disabled"`
	Value string `json:"value,omitempty" maxLength:"1000" description:"data the click submits; empty when omitted"`
	Agent string `json:"agent,omitempty" maxLength:"300" description:"when set, the agent can click this button on the page as the user would; tells the agent what the click does"`
}

// New builds a button widget for Library.Control: a click submits the
// button's value param to the script waiting in ui.draw(). With agent set, the
// button is also an Input of the enclosing space, so the agent can click it.
func New(ctx context.Context, p params, submit func(ctx context.Context, value string) bool) (api.Widget[patch], error) {
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
	state doors.Source[params]
	submit func(ctx context.Context, value string) bool
}

func (b widget) Update(ctx context.Context, patch patch) {
	b.state.Mutate(ctx, func(p params) params {
		if patch.Text != "" {
			p.Text = patch.Text
		}
		if patch.Disabled != nil {
			p.Disabled = *patch.Disabled
		}
		return p
	})
}

//line button.gox:61
func (b widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line button.gox:62
		__e = __c.Any(b.state.Bind(b.render)); if __e != nil { return }
	return })
//line button.gox:63
}

func (p params) input() gox.Proxy {
	if p.Agent == "" {
		return gox.ProxyFunc(func(cur gox.Cursor, el gox.Elem) error {
			return el(cur)
		})
	}
	return neui.Input{Desc: p.Agent}
}

//line button.gox:74
func (b widget) render(p params) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line button.gox:75
		__e = (p.input()).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line button.gox:75
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
//line button.gox:82
					__e = __c.Set("type", "button"); if __e != nil { return }
//line button.gox:83
					__e = __c.Set("class", "neisland-button neisland-button-" + p.Size + " neisland-button-" + p.Style); if __e != nil { return }
//line button.gox:84
					__e = __c.Set("disabled", p.Disabled); if __e != nil { return }
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
