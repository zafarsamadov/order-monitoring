package pkg

import (
	"testing"
	"time"
)

func TestCanonicalJSONIgnoresKeyOrderAndVolatileKeys(t *testing.T) {
	a, ah, err := CanonicalJSON([]byte(`{"b":1, "a":2, "age":10}`), []string{"age"})
	if err != nil {
		t.Fatal(err)
	}
	b, bh, _ := CanonicalJSON([]byte(`{"a":2,"b":1,"age":99}`), []string{"age"})

	if SHA256Hex(ah) != SHA256Hex(bh) {
		t.Fatal("kalit tartibi va ignore qilingan kalit hash'ga ta'sir qilmasligi kerak")
	}
	if string(a) == string(b) {
		t.Fatal("to'liq JSON ignore qilingan kalitni saqlashi kerak")
	}
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	base, max := 100*time.Millisecond, time.Second
	for attempt, want := range map[int]time.Duration{1: 100 * time.Millisecond, 3: 400 * time.Millisecond, 10: time.Second} {
		got := Backoff(attempt, base, max)
		if got < want/2 || got > want {
			t.Errorf("attempt %d: %v, [%v, %v] oralig'ida bo'lishi kerak", attempt, got, want/2, want)
		}
	}
}

func TestTruncateAndEmptyToNil(t *testing.T) {
	if Truncate("abcdef", 3) != "abc..." || Truncate("ab", 3) != "ab" {
		t.Fatal("Truncate noto'g'ri")
	}
	if EmptyToNil("") != nil || EmptyToNil("x") != "x" {
		t.Fatal("EmptyToNil noto'g'ri")
	}
}
