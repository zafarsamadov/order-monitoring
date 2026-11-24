package pkg

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
)

// NetErrorKind tarmoq xatosining turini qisqa matn bilan qaytaradi. Matnda host/URL bo'lmaydi,
// shuning uchun uni logga xavfsiz yozish mumkin.
func NetErrorKind(err error) string {
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
