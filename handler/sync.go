package handler

import (
	"context"
	"errors"
	"net/http"

	"order-monitoring/service"

	"github.com/gin-gonic/gin"
)

// Sync POST /sync — sync'ni ishga tushiradi va tugaguncha kutib, natijani qaytaradi.
// Sync ketayotgan bo'lsa 409.
func (h *Handler) Sync(c *gin.Context) {
	// Mijoz ulanishni uzsa ham sync oxirigacha (yoki xatogacha) davom etadi.
	run, err := h.service.Sync().Run(context.WithoutCancel(c.Request.Context()))
	switch {
	case errors.Is(err, service.ErrAlreadyRunning):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil && run == nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	case err != nil:
		// Qisman natija ham qaytadi: qaysi sahifagacha saqlangani ko'rinadi.
		c.JSON(http.StatusBadGateway, run)
	default:
		c.JSON(http.StatusOK, run)
	}
}

// SyncStatus GET /sync/status — oxirgi sync, oxirgi muvaffaqiyatli sync va hozir ketayotgani. Faqat o'qiydi.
// Stend o'chiq bo'lsa oxirgi sync xato bo'ladi, lekin bazadagi ma'lumot last_success dan.
func (h *Handler) SyncStatus(c *gin.Context) {
	run, err := h.service.Sync().LastRun(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	success, err := h.service.Sync().LastSuccess(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"running":      h.service.Sync().Running(),
		"last_run":     run,
		"last_success": success,
	})
}
