package pkg

// EmptyToNil bo'sh satrni nil (SQL'da NULL) qiladi: "yo'q" holati bitta usulda tekshirilsin.
func EmptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
