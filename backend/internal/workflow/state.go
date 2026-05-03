package workflow

import (
	"fmt"

	"github.com/aravindhsrbk/ims/internal/models"
)

// State defines the behaviour of a single lifecycle phase.
type State interface {
	Status() models.WorkItemStatus
	CanTransitionTo(next models.WorkItemStatus) bool
	// OnExit is a hook called before leaving this state (e.g. RCA validation).
	OnExit(next models.WorkItemStatus, rca *models.RCA) error
}

// --- Concrete states ---

type openState struct{}

func (s *openState) Status() models.WorkItemStatus { return models.StatusOpen }
func (s *openState) CanTransitionTo(next models.WorkItemStatus) bool {
	return next == models.StatusInvestigating
}
func (s *openState) OnExit(_ models.WorkItemStatus, _ *models.RCA) error { return nil }

type investigatingState struct{}

func (s *investigatingState) Status() models.WorkItemStatus { return models.StatusInvestigating }
func (s *investigatingState) CanTransitionTo(next models.WorkItemStatus) bool {
	return next == models.StatusResolved
}
func (s *investigatingState) OnExit(_ models.WorkItemStatus, _ *models.RCA) error { return nil }

type resolvedState struct{}

func (s *resolvedState) Status() models.WorkItemStatus { return models.StatusResolved }
func (s *resolvedState) CanTransitionTo(next models.WorkItemStatus) bool {
	return next == models.StatusClosed
}

// OnExit enforces mandatory RCA before CLOSED is reached.
func (s *resolvedState) OnExit(next models.WorkItemStatus, rca *models.RCA) error {
	if next != models.StatusClosed {
		return nil
	}
	if rca == nil {
		return models.ErrIncompleteRCA
	}
	if rca.FixApplied == "" || rca.PreventionSteps == "" || string(rca.RootCauseCategory) == "" {
		return models.ErrIncompleteRCA
	}
	if rca.IncidentStart.IsZero() || rca.IncidentEnd.IsZero() {
		return models.ErrIncompleteRCA
	}
	if rca.IncidentEnd.Before(rca.IncidentStart) {
		return models.ErrInvalidTimeRange
	}
	return nil
}

type closedState struct{}

func (s *closedState) Status() models.WorkItemStatus                        { return models.StatusClosed }
func (s *closedState) CanTransitionTo(_ models.WorkItemStatus) bool         { return false }
func (s *closedState) OnExit(_ models.WorkItemStatus, _ *models.RCA) error  { return nil }

// --- State machine ---

func newState(status models.WorkItemStatus) State {
	switch status {
	case models.StatusOpen:
		return &openState{}
	case models.StatusInvestigating:
		return &investigatingState{}
	case models.StatusResolved:
		return &resolvedState{}
	case models.StatusClosed:
		return &closedState{}
	default:
		return &openState{}
	}
}

// StateMachine validates and applies Work Item state transitions.
type StateMachine struct{}

// Transition validates the requested transition and returns the MTTR (seconds) if closing.
// rca must be non-nil when transitioning to CLOSED.
func (sm *StateMachine) Transition(wi *models.WorkItem, next models.WorkItemStatus, rca *models.RCA) (mttr *int64, err error) {
	current := newState(wi.Status)

	if !current.CanTransitionTo(next) {
		return nil, fmt.Errorf("%w: %s → %s", models.ErrInvalidTransition, wi.Status, next)
	}
	if err := current.OnExit(next, rca); err != nil {
		return nil, err
	}

	if next == models.StatusClosed && rca != nil {
		seconds := int64(rca.IncidentEnd.Sub(rca.IncidentStart).Seconds())
		mttr = &seconds
	}
	return mttr, nil
}
