package ingestion

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/aravindhsrbk/ims/internal/metrics"
	"github.com/aravindhsrbk/ims/internal/models"
	"github.com/aravindhsrbk/ims/internal/storage/mongodb"
	"github.com/aravindhsrbk/ims/internal/storage/postgres"
	redisstore "github.com/aravindhsrbk/ims/internal/storage/redis"
	"github.com/aravindhsrbk/ims/internal/workflow"
)

// Broadcaster is implemented by the WebSocket hub to fan out state changes.
type Broadcaster interface {
	Broadcast(payload []byte)
}

type Processor struct {
	buffer    *SignalBuffer
	debouncer *Debouncer
	pg        *postgres.Client
	mg        *mongodb.Client
	rd        *redisstore.Client
	alerter   *workflow.AlerterRegistry
	hub       Broadcaster
	metrics   *metrics.Collector
	workers   int
	logger    *log.Logger
}

func NewProcessor(
	buf *SignalBuffer,
	deb *Debouncer,
	pg *postgres.Client,
	mg *mongodb.Client,
	rd *redisstore.Client,
	alerter *workflow.AlerterRegistry,
	hub Broadcaster,
	m *metrics.Collector,
	workers int,
) *Processor {
	return &Processor{
		buffer:    buf,
		debouncer: deb,
		pg:        pg,
		mg:        mg,
		rd:        rd,
		alerter:   alerter,
		hub:       hub,
		metrics:   m,
		workers:   workers,
		logger:    log.New(log.Writer(), "[processor] ", log.LstdFlags),
	}
}

// Run spawns a fixed pool of worker goroutines and blocks until ctx is cancelled.
func (p *Processor) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.worker(ctx)
		}()
	}
	wg.Wait()
}

func (p *Processor) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case signal := <-p.buffer.Chan():
			p.processSignal(ctx, signal)
		}
	}
}

func (p *Processor) processSignal(ctx context.Context, signal *models.Signal) {
	p.metrics.RecordProcessed()

	workItemID, isNew, err := p.debouncer.GetOrCreate(signal.ComponentID, func() (string, error) {
		return p.createWorkItem(ctx, signal)
	})
	if err != nil {
		p.logger.Printf("error creating work item for %s: %v", signal.ComponentID, err)
		return
	}

	signal.WorkItemID = workItemID

	if err := p.mg.InsertSignal(ctx, signal); err != nil {
		p.logger.Printf("error inserting signal %s: %v", signal.ID, err)
	}

	if !isNew {
		if err := p.pg.IncrementSignalCount(ctx, workItemID); err != nil {
			p.logger.Printf("error incrementing signal count for %s: %v", workItemID, err)
		}
	}

	wi, err := p.pg.GetWorkItem(ctx, workItemID)
	if err != nil {
		p.logger.Printf("error fetching work item %s: %v", workItemID, err)
		return
	}

	if err := p.rd.CacheWorkItem(ctx, wi); err != nil {
		p.logger.Printf("error caching work item %s: %v", workItemID, err)
	}

	if isNew {
		p.alerter.Alert(wi)
		p.broadcastWorkItem(wi)
	}
}

func (p *Processor) createWorkItem(ctx context.Context, signal *models.Signal) (string, error) {
	priority, ok := models.ComponentPriority[signal.ComponentType]
	if !ok {
		priority = models.PriorityP3
	}
	wi := &models.WorkItem{
		ID:            uuid.NewString(),
		ComponentID:   signal.ComponentID,
		ComponentType: signal.ComponentType,
		Title:         fmt.Sprintf("[%s] %s – %s", priority, signal.ComponentID, signal.ErrorCode),
		Status:        models.StatusOpen,
		Priority:      priority,
		SignalCount:   1,
		StartTime:     signal.ReceivedAt,
	}
	if err := p.pg.CreateWorkItem(ctx, wi); err != nil {
		return "", err
	}
	return wi.ID, nil
}

func (p *Processor) broadcastWorkItem(wi *models.WorkItem) {
	if p.hub == nil {
		return
	}
	msg := fmt.Sprintf(`{"type":"work_item_update","data":{"id":%q,"status":%q,"priority":%q,"component_id":%q,"signal_count":%d,"updated_at":%q}}`,
		wi.ID, wi.Status, wi.Priority, wi.ComponentID, wi.SignalCount,
		time.Now().Format(time.RFC3339),
	)
	p.hub.Broadcast([]byte(msg))
}

// BroadcastWorkItemUpdate is called by API handlers after status transitions.
func (p *Processor) BroadcastWorkItemUpdate(wi *models.WorkItem) {
	p.broadcastWorkItem(wi)
}

// Debouncer exposes the underlying debouncer so API handlers can update state.
func (p *Processor) Debouncer() *Debouncer { return p.debouncer }
