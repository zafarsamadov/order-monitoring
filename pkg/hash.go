package pkg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// CanonicalJSON kalitlari saralangan JSON qaytaradi: bo'sh joy yoki kalit tartibi o'zgarsa ham
// natija (va hash) bir xil qoladi. Ikkinchi natija — hash uchun, ignore'dagi kalitlarsiz.
func CanonicalJSON(raw []byte, ignore []string) (full, forHash []byte, err error) {
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, nil, err
	}
	full, err = json.Marshal(v) // encoding/json map kalitlarini saralaydi
	if err != nil {
		return nil, nil, err
	}
	if len(ignore) == 0 {
		return full, full, nil
	}
	for _, k := range ignore {
		delete(v, k)
	}
	forHash, err = json.Marshal(v)
	return full, forHash, err
}

// SHA256Hex b ning sha256 hash'ini hex ko'rinishda qaytaradi.
func SHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
