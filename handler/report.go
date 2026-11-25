package handler

import (
	"encoding/csv"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"order-monitoring/web"

	"github.com/gin-gonic/gin"
)

var reportTemplate = template.Must(template.ParseFS(web.FS, "report.html"))

// Report GET /report — hisobot sahifasi: har qoida uchun noyob orderlar soni, 3 ta misol va izoh.
// Sonlar /anomalies bilan bir xil manbadan (service) olinadi.
func (h *Handler) Report(c *gin.Context) {
	resp, err := h.service.Anomaly().GetList(c.Request.Context(), "", "")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := reportTemplate.Execute(c.Writer, gin.H{
		"GeneratedAt": time.Now().Format(time.DateTime),
		"Report":      resp,
	}); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}

// ReportCSV GET /report.csv — har qoida uchun bitta qator:
// rule, severity, orders_count, example_1..3 ("order_code — reason"), comment.
func (h *Handler) ReportCSV(c *gin.Context) {
	resp, err := h.service.Anomaly().GetList(c.Request.Context(), "", "")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="report.csv"`)

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"rule", "severity", "orders_count", "example_1", "example_2", "example_3", "comment"})
	for _, item := range resp.Items {
		examples := make([]string, 3)
		for i, ex := range item.Examples {
			examples[i] = ex.OrderCode + " — " + ex.Reason
		}
		_ = w.Write([]string{
			item.Rule,
			item.Severity,
			strconv.Itoa(item.Total),
			examples[0],
			examples[1],
			examples[2],
			item.Description,
		})
	}
	w.Flush()
}

// AnomaliesCSV GET /anomalies.csv — to'liq ro'yxat: rule, severity, order_code, reason.
func (h *Handler) AnomaliesCSV(c *gin.Context) {
	resp, err := h.service.Anomaly().GetList(c.Request.Context(), "", "")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="anomalies.csv"`)

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"rule", "severity", "order_code", "reason"})
	for _, item := range resp.Items {
		for _, it := range item.Items {
			_ = w.Write([]string{item.Rule, item.Severity, it.OrderCode, it.Reason})
		}
	}
	w.Flush()
}
