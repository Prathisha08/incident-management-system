package workflow

import (
	"fmt"
	"log"
	"time"

	"github.com/aravindhsrbk/ims/internal/models"
)

// AlertStrategy defines the contract for component-specific alerting.
type AlertStrategy interface {
	Alert(wi *models.WorkItem)
	Priority() models.Priority
}

// --- Concrete strategies ---

type baseStrategy struct {
	priority models.Priority
	logger   *log.Logger
}

func (s *baseStrategy) Priority() models.Priority { return s.priority }

func (s *baseStrategy) emit(wi *models.WorkItem, channel string) {
	s.logger.Printf(
		"[ALERT][%s][%s] %s via %s | component=%s | signals=%d | started=%s",
		s.priority, wi.Status, wi.Title, channel, wi.ComponentID,
		wi.SignalCount, wi.StartTime.Format(time.RFC3339),
	)
}

// P0 – RDBMS: immediate page-level alert
type rdbmsStrategy struct{ baseStrategy }

func (s *rdbmsStrategy) Alert(wi *models.WorkItem) {
	s.emit(wi, "PagerDuty+SMS+Slack")
	// In production: POST to PagerDuty Events API v2
}

// P1 – API / MCP_HOST / QUEUE: high-priority alert
type p1Strategy struct{ baseStrategy }

func (s *p1Strategy) Alert(wi *models.WorkItem) {
	s.emit(wi, "Slack+Email")
}

// P2 – Cache: medium-priority alert
type p2Strategy struct{ baseStrategy }

func (s *p2Strategy) Alert(wi *models.WorkItem) {
	s.emit(wi, "Slack")
}

// P3 – NoSQL / unknown: low-priority alert
type p3Strategy struct{ baseStrategy }

func (s *p3Strategy) Alert(wi *models.WorkItem) {
	s.emit(wi, "Email")
}

// --- Registry (factory) ---

// AlerterRegistry maps component types to their alert strategies.
type AlerterRegistry struct {
	strategies map[models.ComponentType]AlertStrategy
	fallback   AlertStrategy
}

func NewAlerterRegistry(logger *log.Logger) *AlerterRegistry {
	mkBase := func(p models.Priority) baseStrategy {
		return baseStrategy{priority: p, logger: logger}
	}
	r := &AlerterRegistry{
		strategies: map[models.ComponentType]AlertStrategy{
			models.ComponentRDBMS:   &rdbmsStrategy{mkBase(models.PriorityP0)},
			models.ComponentAPI:     &p1Strategy{mkBase(models.PriorityP1)},
			models.ComponentMCPHost: &p1Strategy{mkBase(models.PriorityP1)},
			models.ComponentQueue:   &p1Strategy{mkBase(models.PriorityP1)},
			models.ComponentCache:   &p2Strategy{mkBase(models.PriorityP2)},
			models.ComponentNoSQL:   &p3Strategy{mkBase(models.PriorityP3)},
		},
		fallback: &p3Strategy{mkBase(models.PriorityP3)},
	}
	return r
}

// Alert selects the correct strategy for the work item's component type and fires it.
func (r *AlerterRegistry) Alert(wi *models.WorkItem) {
	strategy, ok := r.strategies[wi.ComponentType]
	if !ok {
		strategy = r.fallback
	}
	strategy.Alert(wi)
}

// Register allows runtime extension of the strategy map.
func (r *AlerterRegistry) Register(ct models.ComponentType, strategy AlertStrategy) {
	r.strategies[ct] = strategy
}

// ValidTransitions is a helper the handlers can use to tell clients what moves are legal.
func ValidNextStatuses(current models.WorkItemStatus) []models.WorkItemStatus {
	switch current {
	case models.StatusOpen:
		return []models.WorkItemStatus{models.StatusInvestigating}
	case models.StatusInvestigating:
		return []models.WorkItemStatus{models.StatusResolved}
	case models.StatusResolved:
		return []models.WorkItemStatus{models.StatusClosed}
	default:
		return nil
	}
}

// PriorityLabel returns a human-readable description.
func PriorityLabel(p models.Priority) string {
	switch p {
	case models.PriorityP0:
		return "P0 – Critical"
	case models.PriorityP1:
		return "P1 – High"
	case models.PriorityP2:
		return "P2 – Medium"
	default:
		return fmt.Sprintf("%s – Low", p)
	}
}
