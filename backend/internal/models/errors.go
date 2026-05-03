package models

import "errors"

var (
	ErrIncompleteRCA      = errors.New("RCA is incomplete: all fields required")
	ErrInvalidTimeRange   = errors.New("incident_end must be after incident_start")
	ErrInvalidTransition  = errors.New("invalid state transition")
	ErrWorkItemNotFound   = errors.New("work item not found")
	ErrRCAAlreadyExists   = errors.New("RCA already submitted for this work item")
	ErrBufferFull         = errors.New("signal buffer full, try again later")
)
