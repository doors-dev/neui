package utils

import "strings"

func SanitizeName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	pending := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			if pending && b.Len() > 0 {
				b.WriteByte('_')
			}
			pending = false
			b.WriteRune(r)
		default:
			pending = true
		}
	}
	return b.String()
}
