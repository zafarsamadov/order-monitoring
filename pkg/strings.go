package pkg

// Truncate s ni max baytgacha qisqartiradi va oxiriga "..." qo'shadi (log va xato matnlari uchun).
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
