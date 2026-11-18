// Managed by GoX v0.4.0

//line form_group.gox:1
// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

//line form_group.gox:82
func (f *groupWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_group.gox:83
		if f.item {
//line form_group.gox:84
			for _, field := range f.fields {
//line form_group.gox:85
				__e = __c.Any(field); if __e != nil { return }
			}
		} else  {
//line form_group.gox:88
			__e = (neui.Space{Name: f.params.Name, Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("fieldset"); if __e != nil { return }
				{
//line form_group.gox:88
					__e = __c.Set("class", "neisland-form-box"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_group.gox:89
					if f.params.Label != "" {
						__e = __c.Init("legend"); if __e != nil { return }
						{
//line form_group.gox:90
							__e = __c.Set("class", "neisland-form-box-legend"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line form_group.gox:90
							__e = __c.Any(f.params.Label); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Init("div"); if __e != nil { return }
					{
//line form_group.gox:92
						__e = __c.Set("class", "neisland-form-rail"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_group.gox:93
						for _, field := range f.fields {
//line form_group.gox:94
							__e = __c.Any(field); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
	return })
//line form_group.gox:99
}
