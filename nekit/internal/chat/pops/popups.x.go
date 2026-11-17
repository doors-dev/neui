// Managed by GoX v0.4.0

//line popups.gox:1
package pops

import (
	"context"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

type popState struct {
	seq int
	id any
}

func NewPopups() Popups {
	return Popups{
		state: doors.NewSourceEqual(popState{}, func(old, new popState) bool {
			return old.seq == new.seq
		}),
	}
}

type Popups struct {
	state doors.Source[popState]
}

func (p Popups) Script() gox.Elem {
	return p.state.Bind(func(s popState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line popups.gox:29
			if s.id != nil {
//line popups.gox:30
				__e = __c.Any(p.script(s.seq)); if __e != nil { return }
			}
		return })
//line popups.gox:32
	})
}

//line popups.gox:35
func (p Popups) script(seq int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line popups.gox:36
		__e = (doors.AHook[int]{
		Name: "hide",
		Batch: true,
		On: func(ctx context.Context, r doors.RequestHook[int]) (any, bool) {
			p.state.Mutate(ctx, func(ps popState) popState {
				if ps.seq != r.Data() {
					return ps
				}
				ps.id = nil
				ps.seq += 1
				return ps
			})
			return nil, true
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("script"); if __e != nil { return }
			{
//line popups.gox:50
				__e = __c.Set("data:seq", seq); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const root = document.currentScript.closest(\".nechat-widget\")\n\t\tconst seq = $data(\"seq\")\n\t\tconst fail = (err) => {\n\t\t\tif (err instanceof HookErr)\n\t\t\t\troot.dispatchEvent(new CustomEvent(\"nechat_error\", { detail: err.kind }))\n\t\t}\n\t\tconst hide = (e) => {\n\t\t\tconst pop = e.target.closest?.(\"[data-nechat-pop]\")\n\t\t\tif (pop && root.contains(pop)) return\n\t\t\t$hook(\"hide\", seq).catch(fail)\n\t\t}\n\t\tconst escape = (e) => {\n\t\t\te.preventDefault()\n\t\t\t$hook(\"hide\", seq).catch(fail)\n\t\t}\n\t\tdocument.addEventListener(\"pointerdown\", hide)\n\t\troot.addEventListener(\"nechat_escape\", escape)\n\t\t$sys.clean(() => {\n\t\t\tdocument.removeEventListener(\"pointerdown\", hide)\n\t\t\troot.removeEventListener(\"nechat_escape\", escape)\n\t\t})"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line popups.gox:73
}
