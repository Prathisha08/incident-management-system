package ingestion

import (
	"sync"
	"time"

	"github.com/aravindhsrbk/ims/internal/models"
)

type debounceEntry struct {
	mu         sync.Mutex
	workItemID string
	status     models.WorkItemStatus
	lastSeen   time.Time
	count      int64
}

// Debouncer ensures that signals for the same component ID within the debounce
// window are linked to one Work Item rather than spawning duplicates.
type Debouncer struct {
	items  sync.Map
	window time.Duration
}

func NewDebouncer(window time.Duration) *Debouncer {
	return &Debouncer{window: window}
}

// GetOrCreate returns the active Work Item ID for a component.
// createFn is called exactly once when a new Work Item must be created.
// Returns (workItemID, isNew, error).
func (d *Debouncer) GetOrCreate(componentID string, createFn func() (string, error)) (string, bool, error) {
	raw, _ := d.items.LoadOrStore(componentID, &debounceEntry{})
	e := raw.(*debounceEntry)

	e.mu.Lock()
	defer e.mu.Unlock()

	active := e.workItemID != "" &&
		(e.status == models.StatusOpen || e.status == models.StatusInvestigating) &&
		time.Since(e.lastSeen) < d.window*10 // allow linking beyond window if WI still open

	if active {
		e.count++
		e.lastSeen = time.Now()
		return e.workItemID, false, nil
	}

	// Need a new Work Item
	id, err := createFn()
	if err != nil {
		d.items.Delete(componentID)
		return "", false, err
	}
	e.workItemID = id
	e.status = models.StatusOpen
	e.count = 1
	e.lastSeen = time.Now()
	return id, true, nil
}

// UpdateStatus keeps the debounce map in sync with DB state transitions.
func (d *Debouncer) UpdateStatus(componentID string, status models.WorkItemStatus) {
	if raw, ok := d.items.Load(componentID); ok {
		e := raw.(*debounceEntry)
		e.mu.Lock()
		e.status = status
		e.mu.Unlock()
	}
}

// Evict removes a component entry (e.g. after CLOSED).
func (d *Debouncer) Evict(componentID string) {
	d.items.Delete(componentID)
}
