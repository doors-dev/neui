// Package css holds the nechat stylesheet and its theme overrides. Hosts use
// it through nekit.ChatTheme.
package css

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/zeebo/xxh3"
)

// Theme overrides colours of one face of the nechat palette. Each field takes
// any CSS colour value, such as "#18181b" or "var(--my-bg)"; an empty field
// keeps the default, and a value containing any of {};< is ignored.
type Theme struct {
	// AppBg is the widget background.
	AppBg string
	// AppBorder is the colour of borders and dividers; hover backgrounds derive from it.
	AppBorder string
	// AppInk is the main text colour.
	AppInk string
	// AppInkQuiet is the colour of secondary text, form accents and focus rings.
	AppInkQuiet string
	// PanelBg is the background of the composer, popovers, form fields and message bubbles.
	PanelBg string
	// PanelBorderFocus is the border of input fields, the composer, checkboxes and radio buttons,
	// hovered and pressed buttons, and focused surfaces.
	PanelBorderFocus string
	// ButtonBorder is the border of buttons and the background of pressed buttons and controls.
	ButtonBorder string
	// ButtonInk is the text and icon colour of buttons.
	ButtonInk string
	// ButtonInkOff is the text and icon colour of disabled buttons.
	ButtonInkOff string
	// SecondaryBg is the background of secondary buttons, the context gauge and attached files.
	SecondaryBg string
	// DangerInk is the text colour of destructive actions and errors.
	DangerInk string
	// DangerBgHover is the fill of a hovered destructive button.
	DangerBgHover string
	// DangerBgActive is the fill of a pressed destructive button.
	DangerBgActive string
	// DangerInkOnFill is the text colour on a danger fill.
	DangerInkOnFill string
	// LauncherBg is the fill of the round button that opens a collapsed chat; it defaults to AppInk.
	LauncherBg string
	// LauncherInk is the icon colour of the round button that opens a collapsed chat; it defaults to AppBg.
	LauncherInk string
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
