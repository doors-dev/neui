package utils

import (
	"reflect"
	"runtime/debug"
	"strings"
)

type marker struct{}

var LibPath, LibVersion string = func() (string, string) {
	pkg := reflect.TypeFor[marker]().PkgPath()
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return pkg, "dev"
	}
	best, version := "", "dev"
	check := func(m *debug.Module) {
		if m == nil || m.Path == "" {
			return
		}
		if (pkg == m.Path || strings.HasPrefix(pkg, m.Path+"/")) && len(m.Path) > len(best) {
			best, version = m.Path, m.Version
			if m.Replace != nil && m.Replace.Version != "" {
				version = m.Replace.Version
			}
		}
	}
	check(&info.Main)
	for _, dep := range info.Deps {
		check(dep)
	}
	if best == "" {
		best = pkg
	}
	if version == "" || version == "(devel)" {
		version = "dev"
	}
	return best, version
}()
