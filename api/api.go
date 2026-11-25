package api

import (
	"net/http"

	"order-monitoring/handler"
	"order-monitoring/web"

	"github.com/gin-gonic/gin"
)

func New(h *handler.Handler) *gin.Engine {
	router := gin.Default()

	// frontend
	router.GET("/", h.Index)
	router.StaticFileFS("/static/style.css", "style.css", http.FS(web.FS))
	router.StaticFileFS("/static/app.js", "app.js", http.FS(web.FS))

	{
		sync := router.Group("/sync")

		sync.POST("", h.Sync)
		sync.GET("/status", h.SyncStatus)
	}

	{
		order := router.Group("/orders")

		order.GET("/:code/timeline", h.GetOrderTimeline)
	}

	{
		anomalies := router.Group("/anomalies")

		anomalies.GET("", h.Anomalies)
	}
	return router
}
