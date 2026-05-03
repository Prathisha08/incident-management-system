package api

import (
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/aravindhsrbk/ims/internal/ingestion"
	"github.com/aravindhsrbk/ims/internal/metrics"
	"github.com/aravindhsrbk/ims/internal/storage/mongodb"
	"github.com/aravindhsrbk/ims/internal/storage/postgres"
	redisstore "github.com/aravindhsrbk/ims/internal/storage/redis"
	"github.com/aravindhsrbk/ims/internal/workflow"
)

func NewRouter(
	pg *postgres.Client,
	mg *mongodb.Client,
	rd *redisstore.Client,
	buf *ingestion.SignalBuffer,
	processor *ingestion.Processor,
	alerter *workflow.AlerterRegistry,
	hub *Hub,
	m *metrics.Collector,
	rateLimit float64,
	rateBurst int,
	logger *log.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID", "Retry-After"},
		AllowCredentials: false,
	}))
	r.Use(RequestID())

	h := NewHandler(pg, mg, rd, buf, processor, alerter, m)

	r.GET("/health", h.Health)
	r.GET("/metrics", h.Metrics)

	// WebSocket endpoint (no rate limiting – long-lived connection)
	r.GET("/api/v1/ws", func(c *gin.Context) {
		hub.ServeWS(c.Writer, c.Request)
	})

	v1 := r.Group("/api/v1")

	// Signal ingestion is rate-limited
	v1.POST("/signals", RateLimiter(rateLimit, rateBurst), h.IngestSignal)

	// Work item management
	wi := v1.Group("/work-items")
	wi.GET("", h.ListWorkItems)
	wi.GET("/:id", h.GetWorkItem)
	wi.PATCH("/:id/status", h.TransitionStatus)
	wi.POST("/:id/rca", h.SubmitRCA)
	wi.GET("/:id/signals", h.GetSignals)

	// 404 fallback
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
	})

	logger.Printf("routes registered (rate-limit: %.0f rps, burst: %d)", rateLimit, rateBurst)
	return r
}
