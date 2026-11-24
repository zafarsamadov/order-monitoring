package dona

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"order-monitoring/pkg"
)

// HTTPError — stend 4xx/5xx qaytardi.
type HTTPError struct {
	Path   string
	Status int
	Body   string
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

// isRetryable: tarmoq xatolari, 5xx va 429 — vaqtinchalik. Qolgan 4xx (401 ham) bu yerda
// qayta urinilmaydi; 401 ni Get alohida qayta login bilan hal qiladi.
func isRetryable(err error) bool {
	var netErr *NetError
	if errors.As(err, &netErr) {
		return !errors.Is(err, context.Canceled)
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Status >= 500 || httpErr.Status == http.StatusTooManyRequests
	}
	return false
}
