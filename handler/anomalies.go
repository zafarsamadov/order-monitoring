package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Anomalies GET /anomalies?rule=&severity= — qoidalar natijasi. Ikkala filtr ixtiyoriy.
func (h *Handler) Anomalies(c *gin.Context) {
	severity := c.Query("severity")
	if severity != "" && severity != "low" && severity != "medium" && severity != "high" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "severity: low, medium yoki high bo'lishi kerak"})
		return
	}

	resp, err := h.service.Anomaly().GetList(c.Request.Context(), c.Query("rule"), severity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
