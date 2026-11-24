package pkg

import (
	"context"
	"math/rand/v2"
	"time"
)

// Sleep d davomida kutadi; ctx bekor qilinsa, darhol ctx xatosini qaytaradi.
func Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Backoff attempt-urinish (1 dan boshlab) oldidan kutish vaqti: base, 2*base, 4*base, ... max gacha.
// Jitter: natija [d/2, d] oralig'ida tasodifiy, bir vaqtda qayta urinishlar to'qnashmasligi uchun.
func Backoff(attempt int, base, max time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := base << (attempt - 1)
	if d <= 0 || d > max {
		d = max
	}
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}
