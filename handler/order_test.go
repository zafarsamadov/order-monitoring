package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"order-monitoring/models"
	"order-monitoring/service"
	"order-monitoring/storage"

	"github.com/gin-gonic/gin"
)

// fakeService — faqat Order() ishlatiladi.
type fakeService struct {
	service.ServiceI
	order fakeOrderService
}

func (f fakeService) Order() service.OrderServiceI { return f.order }

type fakeOrderService struct {
	timeline *models.Timeline
	err      error
	gotCode  *string
}

func (f fakeOrderService) Timeline(_ context.Context, code string) (*models.Timeline, error) {
	if f.gotCode != nil {
		*f.gotCode = code
	}
	return f.timeline, f.err
}

func serveTimeline(t *testing.T, svc fakeOrderService, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewHandler(fakeService{order: svc})
	r := gin.New()
	r.GET("/orders/:code/timeline", h.GetOrderTimeline)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestGetOrderTimelineOK(t *testing.T) {
	var gotCode string
	svc := fakeOrderService{
		gotCode: &gotCode,
		timeline: &models.Timeline{
			Order:     models.TimelineOrder{ID: "o1", Code: "AB123", Packages: []models.TimelinePackage{}},
			Events:    []models.TimelineEvent{{Type: "order_created", Source: models.SourceOrder}},
			Anomalies: []models.Anomaly{},
		},
	}

	w := serveTimeline(t, svc, "/orders/AB123/timeline")
	if w.Code != http.StatusOK {
		t.Fatalf("200 kutilgan, bor %d: %s", w.Code, w.Body)
	}
	if gotCode != "AB123" {
		t.Fatalf("repo'ga AB123 kodi berilishi kerak, berildi %q", gotCode)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"order", "events", "anomalies"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("javobda %q yo'q: %s", key, w.Body)
		}
	}
	if string(body["anomalies"]) != "[]" {
		t.Fatalf("anomaliya bo'lmasa [] kutilgan, bor %s", body["anomalies"])
	}
}

func TestGetOrderTimelineNotFound(t *testing.T) {
	w := serveTimeline(t, fakeOrderService{err: storage.ErrOrderNotFound}, "/orders/NOPE/timeline")
	if w.Code != http.StatusNotFound {
		t.Fatalf("404 kutilgan, bor %d", w.Code)
	}
}

func TestGetOrderTimelineError(t *testing.T) {
	w := serveTimeline(t, fakeOrderService{err: errors.New("db yiqildi")}, "/orders/AB123/timeline")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("500 kutilgan, bor %d", w.Code)
	}
}
