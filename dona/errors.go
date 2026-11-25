package dona

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"order-monitoring/pkg"
)

// HTTPError — stend 4xx/5xx qaytardi. RetryAfter — 429/503 dagi Retry-After sarlavhasi (bo'lsa).
type HTTPError struct {
	Path       string
	Status     int
	Body       string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: HTTP %d: %s", e.Path, e.Status, e.Body)
}

// NetError — tarmoq xatosi (timeout, connection refused, ...).
// Error() matnida host/URL yo'q: stend manzili logga chiqmasligi kerak.
type NetError struct {
	Path string
	Err  error
}

func (e *NetError) Error() string {
	return fmt.Sprintf("%s: tarmoq xatosi (%s)", e.Path, pkg.NetErrorKind(e.Err))
}

func (e *NetError) Unwrap() error { return e.Err }

// DecodeError — 200 keldi, lekin tanasi JSON emas yoki kesilgan (masalan proxy sahifasi, uzilish).
type DecodeError struct {
	Path string
	Err  error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("%s: javob JSON emas (%v)", e.Path, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }

// isRetryable: tarmoq xatolari, buzilgan JSON, 5xx va 429 — vaqtinchalik. Qolgan 4xx (401 ham) bu yerda
// qayta urinilmaydi; 401 ni Get alohida qayta login bilan hal qiladi.
func isRetryable(err error) bool {
	var netErr *NetError
	if errors.As(err, &netErr) {
		return !errors.Is(err, context.Canceled)
	}
	var decodeErr *DecodeError
	if errors.As(err, &decodeErr) {
		return true
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Status >= 500 || httpErr.Status == http.StatusTooManyRequests
	}
	return false
}
