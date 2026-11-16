// Managed by GoX v0.4.0

//line form_group.gox:1
package form

import (
	"context"
	"fmt"
	
	"github.com/doors-dev/gox"
	"neui.dev"
)

func newGroupWidget(ctx context.Context, group group, params groupParams) (*groupWidget, error) {
	base, err := group.newGroup(params.Name)
	if err != nil {
		return nil, err
	}
	_, item := group.(*listWidget)
	fields := make([]fieldWidget, 0, len(params.Fields))
	for _, field := range params.Fields {
		widget, err := newFieldWidget(ctx, base, field.v)
		if err != nil {
			return nil, err
		}
		fields = append(fields, widget)
	}
	return &groupWidget{
		item: item,
		params: params,
		group: base,
		fields: fields,
	}, nil
}

type groupWidget struct {
	item bool
	params groupParams
	group *groupBase
	fields []fieldWidget
}

func (f *groupWidget) name() string {
	return f.params.Name
}

func (f *groupWidget) init(ctx context.Context, value any) error {
	var values map[string]any
	if value != nil {
		v, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("group %q: default value is %T, want an object", f.params.Name, value)
		}
		values = v
	}
	for _, field := range f.fields {
		if err := field.init(ctx, values[field.name()]); err != nil {
			return err
		}
	}
	return nil
}

func (f *groupWidget) destroy(ctx context.Context) {
	for _, field := range f.fields {
		field.destroy(ctx)
	}
	f.group.destroy(ctx)
}

//line form_group.gox:68
func (f *groupWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_group.gox:69
		if f.item {
//line form_group.gox:70
			for _, field := range f.fields {
//line form_group.gox:71
				__e = __c.Any(field); if __e != nil { return }
			}
		} else  {
//line form_group.gox:74
			__e = (neui.Space{Name: f.params.Name, Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("fieldset"); if __e != nil { return }
				{
//line form_group.gox:74
					__e = __c.Set("class", "neisland-form-box"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_group.gox:75
					if f.params.Label != "" {
						__e = __c.Init("legend"); if __e != nil { return }
						{
//line form_group.gox:76
							__e = __c.Set("class", "neisland-form-box-legend"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line form_group.gox:76
							__e = __c.Any(f.params.Label); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Init("div"); if __e != nil { return }
					{
//line form_group.gox:78
						__e = __c.Set("class", "neisland-form-rail"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_group.gox:79
						for _, field := range f.fields {
//line form_group.gox:80
							__e = __c.Any(field); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
	return })
//line form_group.gox:85
}
