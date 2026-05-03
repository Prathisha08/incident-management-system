package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aravindhsrbk/ims/internal/ingestion"
	"github.com/aravindhsrbk/ims/internal/metrics"
	"github.com/aravindhsrbk/ims/internal/models"
	"github.com/aravindhsrbk/ims/internal/storage/mongodb"
	"github.com/aravindhsrbk/ims/internal/storage/postgres"
	redisstore "github.com/aravindhsrbk/ims/internal/storage/redis"
	"github.com/aravindhsrbk/ims/internal/workflow"
)

type Handler struct {
	pg        *postgres.Client
	mg        *mongodb.Client
	rd        *redisstore.Client
	buf       *ingestion.SignalBuffer
	processor *ingestion.Processor
	sm        *workflow.StateMachine
	alerter   *workflow.AlerterRegistry
	metrics   *metrics.Collector
}

func NewHandler(
	pg *postgres.Client,
	mg *mongodb.Client,
	rd *redisstore.Client,
	buf *ingestion.SignalBuffer,
	processor *ingestion.Processor,
	alerter *workflow.AlerterRegistry,
	m *metrics.Collector,
) *Handler {
	return &Handler{
		pg:        pg,
		mg:        mg,
		rd:        rd,
		buf:       buf,
		processor: processor,
		sm:        &workflow.StateMachine{},
		alerter:   alerter,
		metrics:   m,
	}
}

// IngestSignal handles POST /api/v1/signals
func (h *Handler) IngestSignal(c *gin.Context) {
	var req models.IngestSignalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	signal := &models.Signal{
		ID:            uuid.NewString(),
		ComponentID:   req.ComponentID,
		ComponentType: req.ComponentType,
		ErrorCode:     req.ErrorCode,
		Message:       req.Message,
		Severity:      req.Severity,
		Metadata:      req.Metadata,
		ReceivedAt:    time.Now(),
	}

	h.metrics.RecordReceived()

	if err := h.buf.Push(signal); err != nil {
		h.metrics.RecordDropped()
		c.Header("Retry-After", "1")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "buffer full – apply backpressure",
			"dropped": h.buf.Dropped(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"id":         signal.ID,
		"status":     "queued",
		"buffer_len": h.buf.Len(),
	})
}

// ListWorkItems handles GET /api/v1/work-items?status=OPEN
func (h *Handler) ListWorkItems(c *gin.Context) {
	statusFilter := c.DefaultQuery("status", "")

	// Hot-path: serve active dashboard from Redis
	if statusFilter == "" || statusFilter == "ACTIVE" {
		if items, err := h.rd.GetDashboard(c.Request.Context()); err == nil && len(items) > 0 {
			c.JSON(http.StatusOK, gin.H{"data": items, "source": "cache"})
			return
		}
	}

	items, err := h.pg.ListWorkItems(c.Request.Context(), statusFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []*models.WorkItem{}
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "source": "db"})
}

// GetWorkItem handles GET /api/v1/work-items/:id
func (h *Handler) GetWorkItem(c *gin.Context) {
	id := c.Param("id")
	wi, err := h.pg.GetWorkItem(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "work item not found"})
		return
	}
	rca, _ := h.pg.GetRCAByWorkItem(c.Request.Context(), id)
	wi.RCA = rca
	c.JSON(http.StatusOK, wi)
}

// TransitionStatus handles PATCH /api/v1/work-items/:id/status
func (h *Handler) TransitionStatus(c *gin.Context) {
	id := c.Param("id")
	var req models.TransitionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wi, err := h.pg.GetWorkItem(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "work item not found"})
		return
	}

	var rca *models.RCA
	if req.Status == models.StatusClosed {
		rca, err = h.pg.GetRCAByWorkItem(c.Request.Context(), id)
		if err != nil || rca == nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "cannot close: RCA not submitted. Submit RCA first via POST /api/v1/work-items/:id/rca",
			})
			return
		}
	}

	mttr, err := h.sm.Transition(wi, req.Status, rca)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	if err := h.pg.UpdateWorkItemStatus(c.Request.Context(), id, req.Status, mttr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	wi.Status = req.Status
	wi.MTTRSeconds = mttr

	_ = h.rd.CacheWorkItem(c.Request.Context(), wi)
	h.processor.Debouncer().UpdateStatus(wi.ComponentID, req.Status)

	if req.Status == models.StatusClosed {
		h.processor.Debouncer().Evict(wi.ComponentID)
	}

	h.processor.BroadcastWorkItemUpdate(wi)

	c.JSON(http.StatusOK, wi)
}

// SubmitRCA handles POST /api/v1/work-items/:id/rca
func (h *Handler) SubmitRCA(c *gin.Context) {
	id := c.Param("id")
	var req models.SubmitRCARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	wi, err := h.pg.GetWorkItem(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "work item not found"})
		return
	}
	if wi.Status != models.StatusResolved {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "RCA can only be submitted when the work item is in RESOLVED state",
		})
		return
	}

	existing, _ := h.pg.GetRCAByWorkItem(c.Request.Context(), id)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": models.ErrRCAAlreadyExists.Error()})
		return
	}

	submittedBy := req.SubmittedBy
	if submittedBy == "" {
		submittedBy = "system"
	}

	rca := &models.RCA{
		ID:                uuid.NewString(),
		WorkItemID:        id,
		IncidentStart:     req.IncidentStart,
		IncidentEnd:       req.IncidentEnd,
		RootCauseCategory: req.RootCauseCategory,
		FixApplied:        req.FixApplied,
		PreventionSteps:   req.PreventionSteps,
		SubmittedBy:       submittedBy,
	}

	if err := h.pg.CreateRCA(c.Request.Context(), rca); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rca)
}

// GetSignals handles GET /api/v1/work-items/:id/signals?limit=50&skip=0
func (h *Handler) GetSignals(c *gin.Context) {
	id := c.Param("id")
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	skip, _ := strconv.ParseInt(c.DefaultQuery("skip", "0"), 10, 64)
	if limit > 200 {
		limit = 200
	}

	signals, err := h.mg.GetSignalsByWorkItem(c.Request.Context(), id, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	total, _ := h.mg.CountSignalsByWorkItem(c.Request.Context(), id)
	if signals == nil {
		signals = []*models.Signal{}
	}
	c.JSON(http.StatusOK, gin.H{"data": signals, "total": total, "limit": limit, "skip": skip})
}

// Health handles GET /health
func (h *Handler) Health(c *gin.Context) {
	ctx := c.Request.Context()
	status := gin.H{"status": "ok", "components": gin.H{}}

	overall := "ok"
	components := map[string]string{}

	if err := h.pg.Ping(ctx); err != nil {
		components["postgres"] = "degraded: " + err.Error()
		overall = "degraded"
	} else {
		components["postgres"] = "ok"
	}

	if err := h.mg.Ping(ctx); err != nil {
		components["mongodb"] = "degraded: " + err.Error()
		overall = "degraded"
	} else {
		components["mongodb"] = "ok"
	}

	if err := h.rd.Ping(ctx); err != nil {
		components["redis"] = "degraded: " + err.Error()
		overall = "degraded"
	} else {
		components["redis"] = "ok"
	}

	status["status"] = overall
	status["components"] = components
	status["buffer"] = gin.H{
		"len":     h.buf.Len(),
		"cap":     h.buf.Cap(),
		"dropped": h.buf.Dropped(),
	}

	code := http.StatusOK
	if overall != "ok" {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, status)
}

// Metrics handles GET /metrics
func (h *Handler) Metrics(c *gin.Context) {
	snap := h.metrics.Snapshot()
	snap["buffer_len"] = int64(h.buf.Len())
	snap["buffer_cap"] = int64(h.buf.Cap())
	c.JSON(http.StatusOK, snap)
}
