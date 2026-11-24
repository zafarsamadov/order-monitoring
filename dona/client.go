// Package dona — DONA stendi klienti. Stendga faqat o'qish uchun murojaat qilinadi:
// yagona POST bu login, qolgan hamma so'rov GET. Klientning ochiq metodlari faqat
// Login va Get; boshqa HTTP method yuboradigan kod yo'q (client_test.go buni tekshiradi).
package dona

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync"
	"time"

	"order-monitoring/logger"
)

const (
	loginPath = "/auth/login"

	// Retry orasidagi kutish: 500ms, 1s, 2s, ... eng ko'pi bilan 10s (+ jitter).
	defaultBaseBackoff = 500 * time.Millisecond
	defaultMaxBackoff  = 10 * time.Second

	maxBodyBytes = 50 << 20
)

type Options struct {
	BaseURL       string // masalan http://host:port/api/v1
	Email         string
	Password      string
	WarehouseCode string // X-Warehouse-Code header, bo'sh bo'lsa yuborilmaydi
	WarehouseID   string // X-Warehouse-Id header, bo'sh bo'lsa yuborilmaydi
	Timeout       time.Duration
	MaxRetries    int
}

type Client struct {
	opt  Options
	http *http.Client

	mu    sync.Mutex
	token string

	baseBackoff time.Duration
	maxBackoff  time.Duration
}

func New(opt Options) *Client {
	return &Client{
		opt:         opt,
		http:        &http.Client{Timeout: opt.Timeout},
		baseBackoff: defaultBaseBackoff,
		maxBackoff:  defaultMaxBackoff,
	}
}

// Login email/parol bilan token oladi. Bu stendga yuboriladigan yagona POST.
func (c *Client) Login(ctx context.Context) error {
	body, err := json.Marshal(map[string]string{"email": c.opt.Email, "password": c.opt.Password})
	if err != nil {
		return err
	}

	var out struct {
		Token string `json:"token"`
	}
	err = c.withRetry(ctx, loginPath, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.opt.BaseURL+loginPath, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, &out)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if out.Token == "" {
		return errors.New("login: javobda token yo'q")
	}

	c.mu.Lock()
	c.token = out.Token
	c.mu.Unlock()
	logger.Info("stendga login qilindi")
	return nil
}

// Get path bo'yicha GET so'rov yuboradi va JSON javobni out'ga yozadi.
// Token bo'lmasa avval login qiladi; 401 kelsa bir marta qayta login qilib takrorlaydi.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	if c.currentToken() == "" {
		if err := c.Login(ctx); err != nil {
			return err
		}
	}

	build := func() (*http.Request, error) {
		u := c.opt.BaseURL + path
		if len(query) > 0 {
			u += "?" + query.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.currentToken())
		return req, nil
	}

	err := c.withRetry(ctx, path, build, out)

	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.Status == http.StatusUnauthorized {
		logger.Warnf("%s: 401, qayta login qilinadi", path)
		if err := c.Login(ctx); err != nil {
			return err
		}
		err = c.withRetry(ctx, path, build, out)
	}
	return err
}

func (c *Client) currentToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

// withRetry vaqtinchalik xatolarda (tarmoq, 5xx, 429) exponential backoff + jitter bilan
// qayta urinadi. Qolgan 4xx darhol qaytariladi.
func (c *Client) withRetry(ctx context.Context, path string, build func() (*http.Request, error), out any) error {
	var lastErr error
	for attempt := 0; attempt <= c.opt.MaxRetries; attempt++ {
		if attempt > 0 {
			wait := c.backoff(attempt)
			logger.Warnf("%v; %d-urinish %s dan keyin", lastErr, attempt+1, wait.Round(time.Millisecond))
			if err := sleep(ctx, wait); err != nil {
				return err
			}
		}

		req, err := build()
		if err != nil {
			return err
		}
		lastErr = c.send(req, path, out)
		if lastErr == nil || !isRetryable(lastErr) {
			return lastErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return fmt.Errorf("%d urinishdan keyin ham bo'lmadi: %w", c.opt.MaxRetries+1, lastErr)
}

func (c *Client) send(req *http.Request, path string, out any) error {
	req.Header.Set("Accept", "application/json")
	if c.opt.WarehouseCode != "" {
		req.Header.Set("X-Warehouse-Code", c.opt.WarehouseCode)
	}
	if c.opt.WarehouseID != "" {
		req.Header.Set("X-Warehouse-Id", c.opt.WarehouseID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return &NetError{Path: path, Err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		// Javob o'rtasida uzilish — tarmoq xatosi, qayta urinsa bo'ladi.
		return &NetError{Path: path, Err: err}
	}

	if resp.StatusCode >= 400 {
		return &HTTPError{Path: path, Status: resp.StatusCode, Body: snippet(body)}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s: javob JSON emas: %w", path, err)
	}
	return nil
}

func (c *Client) backoff(attempt int) time.Duration {
	d := c.baseBackoff << (attempt - 1)
	if d <= 0 || d > c.maxBackoff {
		d = c.maxBackoff
	}
	// Jitter: [d/2, d] oralig'ida tasodifiy, bir vaqtda qayta urinishlar to'qnashmasligi uchun.
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func snippet(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
