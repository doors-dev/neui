package match

import (
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"neui.dev/api"
)

const (
	snippetLead  = 40
	snippetTrail = 120
)

func Fold(text string) string {
	return strings.ToLower(text)
}

func FoldResponse(response api.Indexed[api.Content]) string {
	var b strings.Builder
	for _, i := range slices.Sorted(maps.Keys(response)) {
		text, ok := response[i].(api.ContentText)
		if !ok {
			continue
		}
		b.WriteString(Fold(text.Text))
		b.WriteByte('\n')
	}
	return b.String()
}

func Round(input api.Input, response api.Indexed[api.Content], fold string) (string, bool) {
	if found, ok := snippet(input.Text, fold); ok {
		return found, true
	}
	for _, i := range slices.Sorted(maps.Keys(response)) {
		text, ok := response[i].(api.ContentText)
		if !ok {
			continue
		}
		if found, ok := snippet(text.Text, fold); ok {
			return found, true
		}
	}
	return "", false
}

func snippet(text string, fold string) (string, bool) {
	i := strings.Index(Fold(text), fold)
	if i == -1 {
		return "", false
	}
	start, end := 0, len(text)
	lowered := 0
	for at, r := range text {
		if lowered == i {
			start = at
		}
		if lowered == i+len(fold) {
			end = at
			break
		}
		lowered += utf8.RuneLen(unicode.ToLower(r))
	}
	lo := max(start-snippetLead, 0)
	for lo > 0 && !utf8.RuneStart(text[lo]) {
		lo--
	}
	hi := min(end+snippetTrail, len(text))
	for hi < len(text) && !utf8.RuneStart(text[hi]) {
		hi++
	}
	return text[lo:hi], true
}
