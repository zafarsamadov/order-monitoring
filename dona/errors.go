package dona

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
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
	return fmt.Sprintf("%s: tarmoq xatosi (%s)", e.Path, netKind(e.Err))
}

func (e *NetError) Unwrap() error { return e.Err }

func netKind(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return "bekor qilindi"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused"
	case errors.Is(err, syscall.ECONNRESET):
		return "connection reset"
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.EHOSTDOWN):
		return "host'ga yetib bo'lmadi"
	case errors.Is(err, syscall.ENETUNREACH):
		return "tarmoqqa yetib bo'lmadi"
	case errors.Is(err, syscall.EPERM), errors.Is(err, syscall.EACCES):
		return "ulanishga ruxsat yo'q (firewall/sandbox)"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "ulanish kutilmaganda yopildi"
	default:
		return "boshqa"
	}
}

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
