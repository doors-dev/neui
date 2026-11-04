package core

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

var identifier = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

var reserved = []string{"slot", "draw", "notify", "root"}

func (l *library) checkName(name string) error {
	if !identifier.MatchString(name) {
		return fmt.Errorf("%q: %w", name, errName)
	}
	if slices.Contains(reserved, name) {
		return fmt.Errorf("%q: %w", name, errNameReserved)
	}
	if _, ok := l.widgets[name]; ok {
		return fmt.Errorf("%q: %w", name, errNameTaken)
	}
	if _, ok := l.layouts[name]; ok {
		return fmt.Errorf("%q: %w", name, errNameTaken)
	}
	return nil
}

func writeItem(b *strings.Builder, name string, desc string) {
	b.WriteString(name)
	if desc != "" {
		b.WriteString("  ")
		b.WriteString(desc)
	}
	b.WriteString("\n")
}

func writeSchema(b *strings.Builder, label string, raw []byte) {
	fmt.Fprintf(b, "  %-6s %s\n", label, raw)
}

func (l Library) WriteSpec(b *strings.Builder) {
	names := slices.Sorted(maps.Keys(l.widgets))
	for _, control := range []bool{false, true} {
		first := true
		for _, name := range names {
			f := l.widgets[name]
			if f.control() != control {
				continue
			}
			if first {
				first = false
				if control {
					b.WriteString("\nControls: c = ui.<name>(params); c.update(patch); ui.draw() -> {id: c.id, data}\n")
				} else {
					b.WriteString("\nViews: v = ui.<name>(params); v.update(patch)\n")
				}
			}
			f.writeSpec(b, name)
		}
	}
	if len(l.layouts) > 0 {
		b.WriteString("\nLayouts: ui.<name>(params, ...children)\n")
		for _, name := range slices.Sorted(maps.Keys(l.layouts)) {
			l.layouts[name].writeSpec(b, name)
		}
	}
	fetches := l.fetch.List()
	if len(fetches) > 0 {
		b.WriteString("\nApp functions: app.<name>(input) -> result\n")
		for _, t := range fetches {
			writeItem(b, "app."+t.Name(), t.Desc())
			writeSchema(b, "input", t.Params())
			writeSchema(b, "result", t.Output())
		}
	}
}
