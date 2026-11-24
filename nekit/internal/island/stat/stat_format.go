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

package stat

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

type numberFormat struct {
	kind    string
	unit    string
	symbol  string
	scale   int
	printer *message.Printer
}

func (f numberFormat) show(value float64) string {
	size := math.Abs(value)
	shown := size
	text := ""
	switch f.kind {
	case "duration":
		if milliseconds := math.Round(size * 1000); milliseconds >= 1 && milliseconds < 1000 {
			text = f.decimal(milliseconds, 0, 0) + "ms"
			break
		}
		seconds := math.Round(size)
		shown = seconds
		units := []struct {
			seconds float64
			name    string
		}{{86400, "d"}, {3600, "h"}, {60, "m"}, {1, "s"}}
		for i, u := range units {
			if seconds < u.seconds && u.seconds > 1 {
				continue
			}
			text = f.decimal(math.Floor(seconds/u.seconds), 0, 0) + u.name
			if i+1 < len(units) {
				if rest := math.Floor(math.Mod(seconds, u.seconds) / units[i+1].seconds); rest > 0 {
					text += " " + f.decimal(rest, 0, 0) + units[i+1].name
				}
			}
			break
		}
	case "bytes":
		units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
		scaled, i := size, 0
		for i < len(units)-1 && math.Round(scaled*10)/10 >= 1024 {
			scaled /= 1024
			i++
		}
		shown = math.Round(scaled * 10)
		text = f.decimal(scaled, 0, 1) + " " + units[i]
	default:
		figure, decimals := size, 2
		switch f.kind {
		case "percent":
			figure, decimals = size*100, 1
		case "currency":
			decimals = f.scale
		}
		shown, _ = strconv.ParseFloat(strconv.FormatFloat(figure, 'f', decimals, 64), 64)
		switch {
		case shown >= 10000:
			suffixes := []string{"K", "M", "B"}
			scaled, i := figure/1000, 0
			for i < len(suffixes)-1 && math.Round(scaled*10)/10 >= 1000 {
				scaled /= 1000
				i++
			}
			text = f.decimal(scaled, 0, 1) + suffixes[i]
		case f.kind == "currency":
			text = f.decimal(figure, decimals, decimals)
		default:
			if figure > 0 && figure < 1 {
				decimals = min(6, max(decimals, 1-int(math.Floor(math.Log10(figure)))))
				shown, _ = strconv.ParseFloat(strconv.FormatFloat(figure, 'f', decimals, 64), 64)
			}
			text = f.decimal(figure, 0, decimals)
		}
	}
	switch f.kind {
	case "currency":
		if symbol := []rune(f.symbol); unicode.IsLetter(symbol[len(symbol)-1]) {
			text = " " + text
		}
		text = f.symbol + text
	case "percent":
		text = f.percent(text)
	case "number":
		if f.unit != "" {
			text += " " + f.unit
		}
	}
	if value < 0 && shown > 0 {
		return "-" + text
	}
	return text
}

func (f numberFormat) exact(value float64) string {
	if f.kind == "percent" {
		value *= 100
	}
	value, _ = strconv.ParseFloat(strconv.FormatFloat(value, 'g', 15, 64), 64)
	_, fraction, _ := strings.Cut(strconv.FormatFloat(value, 'f', -1, 64), ".")
	text := f.decimal(math.Abs(value), len(fraction), len(fraction))
	switch f.kind {
	case "currency":
		if symbol := []rune(f.symbol); unicode.IsLetter(symbol[len(symbol)-1]) {
			text = " " + text
		}
		text = f.symbol + text
	case "percent":
		text = f.percent(text)
	case "duration":
		text += " s"
	case "bytes":
		text += " B"
	default:
		if f.unit != "" {
			text += " " + f.unit
		}
	}
	if value < 0 {
		return "-" + text
	}
	return text
}

func (f numberFormat) change(value float64, reference float64) (string, int) {
	direction := 1
	sign := "+"
	if value < reference {
		direction, sign = -1, "-"
	}
	switch {
	case value == reference:
		return f.percent(f.decimal(0, 0, 0)), 0
	case reference == 0:
		return sign + f.show(math.Abs(value-reference)), direction
	}
	return sign + f.percent(f.decimal(math.Abs(value-reference)/math.Abs(reference)*100, 1, 1)), direction
}

func (f numberFormat) decimal(value float64, least int, most int) string {
	return f.printer.Sprint(number.Decimal(value, number.MinFractionDigits(least), number.MaxFractionDigits(most)))
}

func (f numberFormat) percent(text string) string {
	before, after, _ := strings.Cut(f.printer.Sprint(number.Percent(0.5)), f.printer.Sprint(number.Decimal(50)))
	return before + text + after
}
