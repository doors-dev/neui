// Managed by GoX v0.3.4

//line form_group.gox:1
package ui

import (
	"context"
	"fmt"
	
	"github.com/doors-dev/gox"
)

func newFormWidgetGroup(ctx context.Context, group formGroup, params fieldGroup) (*formWidgetGroup, error) {
	base, err := group.newGroup(params.Name)
	if err != nil {
		return nil, err
	}
	_, item := group.(*formWidgetList)
	fields := make([]formWidgetField, 0, len(params.Fields))
	for _, field := range params.Fields {
		widget, err := newFormWidgetField(ctx, base, field.v)
		if err != nil {
			return nil, err
		}
		fields = append(fields, widget)
	}
	return &formWidgetGroup{
		item: item,
		params: params,
		group: base,
		fields: fields,
	}, nil
}

type formWidgetGroup struct {
	item bool
	params fieldGroup
	group *formGroupBase
	fields []formWidgetField
}

func (f *formWidgetGroup) name() string {
	return f.params.Name
}

func (f *formWidgetGroup) init(ctx context.Context, value any) error {
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

func (f *formWidgetGroup) destroy(ctx context.Context) {
	for _, field := range f.fields {
		field.destroy(ctx)
	}
	f.group.destroy(ctx)
}

//line form_group.gox:67
func (f *formWidgetGroup) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_group.gox:68
		if f.item {
//line form_group.gox:69
			for _, field := range f.fields {
//line form_group.gox:70
				__e = __c.Any(field); if __e != nil { return }
			}
		} else  {
			__e = __c.Init("fieldset"); if __e != nil { return }
			{
//line form_group.gox:73
				__e = __c.Set("class", "kisland-form-box"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_group.gox:74
				if f.params.Label != "" {
					__e = __c.Init("legend"); if __e != nil { return }
					{
//line form_group.gox:75
						__e = __c.Set("class", "kisland-form-box-legend"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_group.gox:75
						__e = __c.Any(f.params.Label); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Init("div"); if __e != nil { return }
				{
//line form_group.gox:77
					__e = __c.Set("class", "kisland-form-rail"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_group.gox:78
					for _, field := range f.fields {
//line form_group.gox:79
						__e = __c.Any(field); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line form_group.gox:84
}
