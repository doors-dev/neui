// Managed by GoX v0.3.4

//line form_file.gox:1
package ui

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

func newFormWidgetFile(ctx context.Context, group formGroup, params fileInput) (*formWidgetFile, error) {
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &formWidgetFile{
		id: doors.IDRand(),
		field: field,
		params: params,
		chosen: doors.NewSource(&[]formFileChosen{}),
	}, nil
}

type formFileChosen struct {
	Name string `json:"name"`
	Size int64 `json:"size"`
	Type string `json:"type"`
}

type formWidgetFile struct {
	id string
	field formGroupField
	params fileInput
	chosen doors.Source[*[]formFileChosen]
	invalid doors.Setter
}

func (f *formWidgetFile) name() string {
	return f.params.Name
}

func (f *formWidgetFile) inputName() string {
	return f.id
}

func (f *formWidgetFile) dir() string {
	return path.Join("island", f.id)
}

func (f *formWidgetFile) init(ctx context.Context, value any) error {
	if value != nil {
		return fmt.Errorf("field %q: default value is %T, a file field can't be prefilled", f.params.Name, value)
	}
	f.setLocked(ctx, nil, f.validate(nil))
	f.field.parent.form().files[f.id] = f
	return nil
}

func (f *formWidgetFile) destroy(ctx context.Context) {
	delete(f.field.parent.form().files, f.id)
	f.field.destroy(ctx)
}

func (f *formWidgetFile) update(ctx context.Context, files []formFileChosen) {
	files = slices.DeleteFunc(files, func(file formFileChosen) bool {
		return file.Name == "" || file.Name == "." || file.Name == ".." || strings.ContainsAny(file.Name, "/\\")
	})
	err := f.validate(files)
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	f.setLocked(ctx, files, err)
	f.chosen.Update(ctx, &files)
	doors.Call(ctx, f.invalid.Set("aria-invalid", formAria(formShownErr(err))))
}

func (f *formWidgetFile) setLocked(ctx context.Context, files []formFileChosen, err error) {
	var value any
	switch {
	case len(files) == 0:
	case f.params.Multiple:
		paths := make([]string, len(files))
		for i, file := range files {
			paths[i] = path.Join(f.dir(), file.Name)
		}
		value = paths
	default:
		value = path.Join(f.dir(), files[0].Name)
	}
	f.field.set(ctx, value, err)
}

func formFileSize(size int64) string {
	switch {
	case size < 1000:
		return fmt.Sprintf("%d B", size)
	case size < 1000*1000:
		return fmt.Sprintf("%d KB", (size + 500) / 1000)
	default:
		return fmt.Sprintf("%.1f MB", float64(size) / 1e6)
	}
}

//line form_file.gox:107
func (f *formWidgetFile) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_file.gox:108
			__e = __c.Set("class", "kisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line form_file.gox:109
				__e = __c.Set("class", "kisland-form-label"); if __e != nil { return }
//line form_file.gox:109
				__e = __c.Set("id", f.id + "-l"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:110
				__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_file.gox:111
				__e = __c.Any(f.field.error.RouteBeam(
				doors.RouteBeamMatch(func(err error) bool {
					return f.params.Required && errors.Is(err, errRequired)
				}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:114
						__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_file.gox:114
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("*"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:114
				return }))),
			)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("label"); if __e != nil { return }
			{
//line form_file.gox:117
				__e = __c.Set("class", "kisland-form-file"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.InitVoid("input"); if __e != nil { return }
				{
//line form_file.gox:119
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_file.gox:120
					__e = __c.Set("class", "kisland-form-file-input"); if __e != nil { return }
//line form_file.gox:121
					__e = __c.Set("id", f.id); if __e != nil { return }
//line form_file.gox:122
					__e = __c.Set("type", "file"); if __e != nil { return }
//line form_file.gox:123
					__e = __c.Set("name", f.id); if __e != nil { return }
//line form_file.gox:124
					__e = __c.Set("accept", f.params.Accept); if __e != nil { return }
//line form_file.gox:125
					__e = __c.Set("multiple", f.params.Multiple); if __e != nil { return }
//line form_file.gox:126
					__e = __c.Set("aria-required", formAria(f.params.Required)); if __e != nil { return }
//line form_file.gox:127
					__e = __c.Set("aria-invalid", formAria(formShownErr(f.field.error.Get()))); if __e != nil { return }
//line form_file.gox:128
					__e = __c.Set("aria-labelledby", f.id + "-l"); if __e != nil { return }
//line form_file.gox:129
					__e = __c.Set("aria-describedby", f.id + "-s " + f.id + "-e"); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line form_file.gox:130
					__e = __c.Set("class", "kisland-form-file-button"); if __e != nil { return }
//line form_file.gox:130
					__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:131
					__e = __c.Any(f.chosen.Bind(func(files *[]formFileChosen) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line form_file.gox:132
						if f.params.Multiple && len(*files) != 0 {
							__e = __c.Text("Add files"); if __e != nil { return }
//line form_file.gox:134
						} else if f.params.Multiple {
							__e = __c.Text("Choose files"); if __e != nil { return }
//line form_file.gox:136
						} else if len(*files) != 0 {
							__e = __c.Text("Replace"); if __e != nil { return }
						} else  {
							__e = __c.Text("Choose file"); if __e != nil { return }
						}
					return })
//line form_file.gox:141
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line form_file.gox:143
					__e = __c.Set("class", "kisland-form-file-summary"); if __e != nil { return }
//line form_file.gox:143
					__e = __c.Set("id", f.id + "-s"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:144
					__e = __c.Any(f.chosen.Bind(func(files *[]formFileChosen) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line form_file.gox:146
						total := int64(0)
					for _, file := range *files {
						total += file.Size
					}

//line form_file.gox:151
						if len(*files) == 0 && f.params.Multiple {
							__e = __c.Text("No files chosen"); if __e != nil { return }
//line form_file.gox:153
						} else if len(*files) == 0 {
							__e = __c.Text("No file chosen"); if __e != nil { return }
//line form_file.gox:155
						} else if !f.params.Multiple {
							__e = __c.Init("span"); if __e != nil { return }
							{
//line form_file.gox:156
								__e = __c.Set("class", "kisland-form-file-name"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:156
								__e = __c.Any((*files)[0].Name); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_file.gox:157
							__e = __c.Many(" · ", formFileSize(total)); if __e != nil { return }
//line form_file.gox:158
						} else if len(*files) == 1 {
//line form_file.gox:159
							__e = __c.Many("1 file · ", formFileSize(total)); if __e != nil { return }
						} else  {
//line form_file.gox:161
							__e = __c.Many(len(*files), " files · ", formFileSize(total)); if __e != nil { return }
						}
					return })
//line form_file.gox:163
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_file.gox:166
			__e = __c.Any(f.chosen.Bind(func(files *[]formFileChosen) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line form_file.gox:167
				if f.params.Multiple && len(*files) != 0 {
					__e = __c.Init("ul"); if __e != nil { return }
					{
//line form_file.gox:168
						__e = __c.Set("class", "kisland-form-file-list"); if __e != nil { return }
//line form_file.gox:168
						__e = __c.Set("aria-label", "Chosen files"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:169
						for _, file := range *files {
							__e = __c.Init("li"); if __e != nil { return }
							{
//line form_file.gox:170
								__e = __c.Set("class", "kisland-form-file-item"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("span"); if __e != nil { return }
								{
//line form_file.gox:171
									__e = __c.Set("class", "kisland-form-file-name"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:171
									__e = __c.Any(file.Name); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
								__e = __c.Init("span"); if __e != nil { return }
								{
//line form_file.gox:172
									__e = __c.Set("class", "kisland-form-file-meta"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:172
									__e = __c.Any(formFileSize(file.Size)); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
								__e = __c.Init("button"); if __e != nil { return }
								{
//line form_file.gox:174
									__e = __c.Set("type", "button"); if __e != nil { return }
//line form_file.gox:175
									__e = __c.Set("class", "kisland-form-remove"); if __e != nil { return }
//line form_file.gox:176
									__e = __c.Set("data-file-remove", file.Name); if __e != nil { return }
//line form_file.gox:177
									__e = __c.Set("aria-label", "Remove " + file.Name); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Text("×"); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
			return })
//line form_file.gox:182
		})); if __e != nil { return }
//line form_file.gox:183
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(formShownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_file.gox:185
						__e = __c.Set("class", "kisland-form-error"); if __e != nil { return }
//line form_file.gox:185
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:185
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:185
				return })
			})),
		)); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
//line form_file.gox:189
				__e = __c.Modify(doors.AHook[[]formFileChosen]{
				Name: "files",
				Scope: f.field.scope(),
				On: func(ctx context.Context, r doors.RequestHook[[]formFileChosen]) (any, bool) {
					f.update(ctx, r.Data())
					return nil, false
				},
			}); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const root = document.currentScript.parentElement\n\t\t\tconst input = root.querySelector(\".kisland-form-file-input\")\n\t\t\tlet files = []\n\t\t\tconst send = async (next) => {\n\t\t\t\tconst prev = files\n\t\t\t\tfiles = next\n\t\t\t\tconst transfer = new DataTransfer()\n\t\t\t\tfor (const file of files) {\n\t\t\t\t\ttransfer.items.add(file)\n\t\t\t\t}\n\t\t\t\tinput.files = transfer.files\n\t\t\t\ttry {\n\t\t\t\t\tawait $hook(\"files\", files.map((file) => ({ name: file.name, size: file.size, type: file.type })))\n\t\t\t\t} catch {\n\t\t\t\t\tfiles = prev\n\t\t\t\t\tconst restore = new DataTransfer()\n\t\t\t\t\tfor (const file of files) {\n\t\t\t\t\t\trestore.items.add(file)\n\t\t\t\t\t}\n\t\t\t\t\tinput.files = restore.files\n\t\t\t\t}\n\t\t\t}\n\t\t\tinput.addEventListener(\"change\", () => {\n\t\t\t\tconst picked = Array.from(input.files)\n\t\t\t\tif (!input.multiple) {\n\t\t\t\t\tsend(picked)\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tsend(files.filter((file) => !picked.some((p) => p.name === file.name)).concat(picked))\n\t\t\t})\n\t\t\troot.addEventListener(\"click\", (event) => {\n\t\t\t\tconst button = event.target.closest(\"[data-file-remove]\")\n\t\t\t\tif (!button) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tsend(files.filter((file) => file.name !== button.dataset.fileRemove))\n\t\t\t})"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_file.gox:236
}

func (f *formWidgetFile) validate(files []formFileChosen) error {
	if len(files) == 0 {
		if f.params.Required {
			return errRequired
		}
		return nil
	}
	if !f.params.Multiple && len(files) > 1 {
		return errors.New("choose one file")
	}
	if f.params.Accept == "" {
		return nil
	}
	for _, file := range files {
		accepted := false
		for _, token := range strings.Split(f.params.Accept, ",") {
			token = strings.ToLower(strings.TrimSpace(token))
			switch {
			case token == "":
			case strings.HasPrefix(token, "."):
				accepted = strings.HasSuffix(strings.ToLower(file.Name), token)
			case strings.HasSuffix(token, "/*"):
				accepted = strings.HasPrefix(strings.ToLower(file.Type), strings.TrimSuffix(token, "*"))
			default:
				accepted = strings.ToLower(file.Type) == token
			}
			if accepted {
				break
			}
		}
		if !accepted {
			return fmt.Errorf("%s isn't an accepted file type, choose %s", file.Name, f.params.Accept)
		}
	}
	return nil
}
