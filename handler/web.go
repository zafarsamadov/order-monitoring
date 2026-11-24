package handler

import (
	"net/http"

	"order-monitoring/web"

	"github.com/gin-gonic/gin"
)

// Index GET / — frontend sahifasi (go:embed orqali binary ichida).
func (h *Handler) Index(c *gin.Context) {
	page, err := web.FS.ReadFile("index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "index.html topilmadi")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", page)
}
