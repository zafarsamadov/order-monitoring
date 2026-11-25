package handler

import (
	"errors"
	"net/http"

	"order-monitoring/storage"

	"github.com/gin-gonic/gin"
)

// GetOrderTimeline GET /orders/:code/timeline — order hodisalari vaqt bo'yicha va anomaliyalari.
// Faqat bazadan o'qiydi. Order topilmasa 404.
func (h *Handler) GetOrderTimeline(c *gin.Context) {
	timeline, err := h.service.Order().Timeline(c.Request.Context(), c.Param("code"))
	switch {
	case errors.Is(err, storage.ErrOrderNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusOK, timeline)
	}
}
