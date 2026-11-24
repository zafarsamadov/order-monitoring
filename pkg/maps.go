package pkg

import "sort"

// SortedKeys map kalitlarini alifbo tartibida qaytaradi (barqaror tartib uchun).
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
