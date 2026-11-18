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
