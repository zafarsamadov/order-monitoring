package api

import (
	"order-monitoring/handler"

	"github.com/gin-gonic/gin"
)

func New(h *handler.Handler) *gin.Engine {
	router := gin.Default()

	{
		sync := router.Group("/sync")

		sync.POST("", h.Sync)
		sync.GET("/status", h.SyncStatus)
	}

	return router
}
