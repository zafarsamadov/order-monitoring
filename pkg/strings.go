package pkg

import "strings"

// Truncate s ni max baytgacha qisqartiradi va oxiriga "..." qo'shadi (log va xato matnlari uchun).
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// JoinNonEmpty bo'sh bo'lmagan qismlarni sep bilan birlashtiradi.
func JoinNonEmpty(sep string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}
