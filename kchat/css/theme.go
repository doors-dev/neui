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
	DangerBgActive   string
	DangerInkOnFill  string
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
		{"danger-bg-active", t.DangerBgActive},
		{"danger-ink-on-fill", t.DangerInkOnFill},
	} {
		if token[1] == "" || strings.ContainsAny(token[1], "{};<") {
			continue
		}
		b.WriteString("--kchat-")
		b.WriteString(face)
		b.WriteString("-")
		b.WriteString(token[0])
		b.WriteString(":")
		b.WriteString(token[1])
		b.WriteString(";")
	}
}

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
