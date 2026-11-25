package handler

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"order-monitoring/models"
	"order-monitoring/service"

	"github.com/gin-gonic/gin"
)

type fakeAnomalyService struct {
	resp *models.RuleResponse
	err  error
}

func (f fakeAnomalyService) GetList(context.Context, string, string) (*models.RuleResponse, error) {
	return f.resp, f.err
}

func serve(t *testing.T, svc fakeAnomalyService, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewHandler(fakeService{anomaly: svc})
	r := gin.New()
	r.GET("/anomalies", h.Anomalies)
	r.GET("/report.csv", h.ReportCSV)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestAnomaliesBadSeverity(t *testing.T) {
	w := serve(t, fakeAnomalyService{}, "/anomalies?severity=urgent")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("400 kutilgan, bor %d", w.Code)
	}
}

func TestAnomaliesUnknownRule(t *testing.T) {
	err := fmt.Errorf("%w %q", service.ErrUnknownRule, "nope")
	w := serve(t, fakeAnomalyService{err: err}, "/anomalies?rule=nope")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "nope") {
		t.Fatalf("400 va xabarda rule nomi kutilgan, bor %d: %s", w.Code, w.Body)
	}
}

// /report.csv: sarlavha va har qoida uchun bitta qator; misollar bo'sh bo'lsa ustun bo'sh qoladi.
func TestReportCSVOneRowPerRule(t *testing.T) {
	svc := fakeAnomalyService{resp: &models.RuleResponse{Items: []models.RuleItem{
		{Rule: "rule_a", Severity: "high", Description: "izoh a", Total: 5, Examples: []models.Item{
			{OrderCode: "A", Reason: "r1"}, {OrderCode: "B", Reason: "r2"}, {OrderCode: "C", Reason: "r3"},
		}},
		{Rule: "rule_b", Severity: "low", Description: "izoh b", Total: 0, Examples: []models.Item{}},
	}}}

	w := serve(t, svc, "/report.csv")
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("200 va text/csv kutilgan, bor %d %s", w.Code, w.Header().Get("Content-Type"))
	}

	rows, err := csv.NewReader(w.Body).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"rule", "severity", "orders_count", "example_1", "example_2", "example_3", "comment"},
		{"rule_a", "high", "5", "A — r1", "B — r2", "C — r3", "izoh a"},
		{"rule_b", "low", "0", "", "", "", "izoh b"},
	}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("CSV:\n%v\nkutilgan:\n%v", rows, want)
	}
}
