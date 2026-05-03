package models

import "time"

type WorkItemStatus string

const (
	StatusOpen          WorkItemStatus = "OPEN"
	StatusInvestigating WorkItemStatus = "INVESTIGATING"
	StatusResolved      WorkItemStatus = "RESOLVED"
	StatusClosed        WorkItemStatus = "CLOSED"
)

type Priority string

const (
	PriorityP0 Priority = "P0"
	PriorityP1 Priority = "P1"
	PriorityP2 Priority = "P2"
	PriorityP3 Priority = "P3"
)

var ComponentPriority = map[ComponentType]Priority{
	ComponentRDBMS:   PriorityP0,
	ComponentAPI:     PriorityP1,
	ComponentMCPHost: PriorityP1,
	ComponentQueue:   PriorityP1,
	ComponentCache:   PriorityP2,
	ComponentNoSQL:   PriorityP3,
}

type WorkItem struct {
	ID            string         `json:"id" db:"id"`
	ComponentID   string         `json:"component_id" db:"component_id"`
	ComponentType ComponentType  `json:"component_type" db:"component_type"`
	Title         string         `json:"title" db:"title"`
	Status        WorkItemStatus `json:"status" db:"status"`
	Priority      Priority       `json:"priority" db:"priority"`
	SignalCount   int64          `json:"signal_count" db:"signal_count"`
	StartTime     time.Time      `json:"start_time" db:"start_time"`
	ResolvedAt    *time.Time     `json:"resolved_at,omitempty" db:"resolved_at"`
	ClosedAt      *time.Time     `json:"closed_at,omitempty" db:"closed_at"`
	MTTRSeconds   *int64         `json:"mttr_seconds,omitempty" db:"mttr_seconds"`
	RCA           *RCA           `json:"rca,omitempty" db:"-"`
	CreatedAt     time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at" db:"updated_at"`
}

type TransitionStatusRequest struct {
	Status WorkItemStatus `json:"status" binding:"required"`
}
