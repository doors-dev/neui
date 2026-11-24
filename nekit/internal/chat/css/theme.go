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

// Package css holds the nechat stylesheet and its theme overrides. Hosts use
// it through nekit.ChatTheme.
package css

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/zeebo/xxh3"
)

type Theme struct {
	AppBg            string
	AppBorder        string
	AppInk           string
	AppInkQuiet      string
	PanelBg          string
	PanelBorderFocus string
	ButtonBorder     string
	ButtonInk        string
	ButtonInkOff     string
	SecondaryBg      string
	DangerInk        string
	DangerBgHover    string
	DangerBgActive   string
	DangerInkOnFill  string
	LauncherBg       string
	LauncherInk      string
}

func (t Theme) empty() bool {
	return t == Theme{}
}

func (t Theme) write(b *bytes.Buffer, face string) {
	for _, token := range [][2]string{
		{"app-bg", t.AppBg},
		{"app-border", t.AppBorder},
		{"app-ink", t.AppInk},
		{"app-ink-quiet", t.AppInkQuiet},
		{"panel-bg", t.PanelBg},
		{"panel-border-focus", t.PanelBorderFocus},
		{"button-border", t.ButtonBorder},
		{"button-ink", t.ButtonInk},
		{"button-ink-off", t.ButtonInkOff},
		{"secondary-bg", t.SecondaryBg},
		{"danger-ink", t.DangerInk},
		{"danger-bg-hover", t.DangerBgHover},
		{"danger-bg-active", t.DangerBgActive},
		{"danger-ink-on-fill", t.DangerInkOnFill},
		{"launcher-bg", t.LauncherBg},
		{"launcher-ink", t.LauncherInk},
	} {
		if token[1] == "" || strings.ContainsAny(token[1], "{};<") {
			continue
		}
		b.WriteString("--nechat-")
		b.WriteString(face)
		b.WriteString("-")
		b.WriteString(token[0])
		b.WriteString(":")
		b.WriteString(token[1])
		b.WriteString(";")
	}
}

// FaceKey returns a short hash identifying the overrides in dark and light,
// or "" when neither sets a usable colour.
func FaceKey(dark Theme, light Theme) string {
	if dark.empty() && light.empty() {
		return ""
	}
	var b bytes.Buffer
	dark.write(&b, "dark")
	light.write(&b, "light")
	if b.Len() == 0 {
		return ""
	}
	return strconv.FormatUint(xxh3.Hash(b.Bytes()), 36)
}
