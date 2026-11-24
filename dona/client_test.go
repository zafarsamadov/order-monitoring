package dona

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeStand — soxta stend: login token beradi, qolgan so'rovlarga handler javob beradi.
// Kelgan har bir so'rov (method + path) yoziladi.
type fakeStand struct {
	mu       sync.Mutex
	requests []string
	logins   int
	handler  func(w http.ResponseWriter, r *http.Request, call int)
	calls    int
}

func (f *fakeStand) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	if r.URL.Path == "/auth/login" {
		f.logins++
		f.mu.Unlock()
		w.Write([]byte(`{"token":"tok"}`))
		return
	}
	f.calls++
	call := f.calls
	f.mu.Unlock()
	f.handler(w, r, call)
}

func newTestClient(t *testing.T, f *fakeStand) *Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	c := New(Options{BaseURL: srv.URL, Email: "e", Password: "p", Timeout: 2 * time.Second, MaxRetries: 3})
	c.baseBackoff = time.Millisecond
	c.maxBackoff = 5 * time.Millisecond
	return c
}

func TestGetRetriesOn500(t *testing.T) {
	f := &fakeStand{handler: func(w http.ResponseWriter, r *http.Request, call int) {
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}}
	c := newTestClient(t, f)

	var out struct{ OK bool }
	if err := c.Get(context.Background(), "/orders", nil, &out); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !out.OK || f.calls != 2 {
		t.Fatalf("kutilgan: 2 urinish va ok=true, bo'ldi: %d urinish, ok=%v", f.calls, out.OK)
	}
}

func TestGetRetriesOn429AndGivesUp(t *testing.T) {
	f := &fakeStand{handler: func(w http.ResponseWriter, r *http.Request, call int) {
		w.WriteHeader(http.StatusTooManyRequests)
	}}
	c := newTestClient(t, f)

	err := c.Get(context.Background(), "/orders", nil, &struct{}{})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 429 {
		t.Fatalf("429 xato kutilgan edi, bo'ldi: %v", err)
	}
	if f.calls != 4 { // 1 + MaxRetries(3)
		t.Fatalf("4 urinish kutilgan edi, bo'ldi %d", f.calls)
	}
}

func TestGetDoesNotRetry4xx(t *testing.T) {
	f := &fakeStand{handler: func(w http.ResponseWriter, r *http.Request, call int) {
		w.WriteHeader(http.StatusNotFound)
	}}
	c := newTestClient(t, f)

	err := c.Get(context.Background(), "/orders", nil, &struct{}{})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 404 {
		t.Fatalf("404 kutilgan edi, bo'ldi: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("4xx qayta urinilmasligi kerak, urinishlar: %d", f.calls)
	}
}

func TestGetReloginOn401(t *testing.T) {
	f := &fakeStand{handler: func(w http.ResponseWriter, r *http.Request, call int) {
		if call == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("Authorization header noto'g'ri: %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{}`))
	}}
	c := newTestClient(t, f)

	if err := c.Get(context.Background(), "/orders", nil, &struct{}{}); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if f.logins != 2 {
		t.Fatalf("401 dan keyin qayta login kutilgan edi, loginlar: %d", f.logins)
	}
}

func TestConnectionRefusedIsRetriedAndHidesHost(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.URL
	srv.Close() // endi port yopiq: connection refused

	c := New(Options{BaseURL: addr, Timeout: time.Second, MaxRetries: 2})
	c.baseBackoff = time.Millisecond
	c.token = "tok"

	err := c.Get(context.Background(), "/orders", nil, &struct{}{})
	var netErr *NetError
	if !errors.As(err, &netErr) {
		t.Fatalf("NetError kutilgan edi, bo'ldi: %v", err)
	}
	host := strings.TrimPrefix(addr, "http://")
	if strings.Contains(err.Error(), host) {
		t.Fatalf("xato matnida stend manzili bo'lmasligi kerak: %v", err)
	}
}

// Stendga faqat GET va login (POST /auth/login) yuborilishi.
func TestOnlyGetAndLoginRequests(t *testing.T) {
	f := &fakeStand{handler: func(w http.ResponseWriter, r *http.Request, call int) {
		w.Write([]byte(`{}`))
	}}
	c := newTestClient(t, f)
	ctx := context.Background()

	if err := c.Login(ctx); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/orders", "/packages", "/inbound/handovers", "/shelves"} {
		if err := c.Get(ctx, p, nil, &struct{}{}); err != nil {
			t.Fatal(err)
		}
	}

	for _, req := range f.requests {
		if req != "POST /auth/login" && !strings.HasPrefix(req, "GET ") {
			t.Errorf("ruxsat etilmagan so'rov: %s", req)
		}
	}
}

// Klientning ochiq API'si faqat Login va Get: yozuvchi metod qo'shilsa, test yiqiladi.
func TestClientExposesOnlyLoginAndGet(t *testing.T) {
	typ := reflect.TypeOf(&Client{})
	var names []string
	for i := 0; i < typ.NumMethod(); i++ {
		names = append(names, typ.Method(i).Name)
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"Get", "Login"}) {
		t.Fatalf("Client ochiq metodlari faqat Get va Login bo'lishi kerak, bor: %v", names)
	}
}
