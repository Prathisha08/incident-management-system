package models

import "time"

type RootCauseCategory string

const (
	RCACategoryInfrastructure  RootCauseCategory = "Infrastructure Failure"
	RCACategorySoftwareBug     RootCauseCategory = "Software Bug"
	RCACategoryConfiguration   RootCauseCategory = "Configuration Error"
	RCACategoryNetwork         RootCauseCategory = "Network Issue"
	RCACategoryCapacity        RootCauseCategory = "Capacity / Scaling Issue"
	RCACategoryThirdParty      RootCauseCategory = "Third-party Service Failure"
	RCACategoryHumanError      RootCauseCategory = "Human Error"
	RCACategoryUnknown         RootCauseCategory = "Unknown"
)

type RCA struct {
	ID                string            `json:"id" db:"id"`
	WorkItemID        string            `json:"work_item_id" db:"work_item_id"`
	IncidentStart     time.Time         `json:"incident_start" db:"incident_start"`
	IncidentEnd       time.Time         `json:"incident_end" db:"incident_end"`
	RootCauseCategory RootCauseCategory `json:"root_cause_category" db:"root_cause_category"`
	FixApplied        string            `json:"fix_applied" db:"fix_applied"`
	PreventionSteps   string            `json:"prevention_steps" db:"prevention_steps"`
	SubmittedBy       string            `json:"submitted_by" db:"submitted_by"`
	SubmittedAt       time.Time         `json:"submitted_at" db:"submitted_at"`
}

type SubmitRCARequest struct {
	IncidentStart     time.Time         `json:"incident_start" binding:"required"`
	IncidentEnd       time.Time         `json:"incident_end" binding:"required"`
	RootCauseCategory RootCauseCategory `json:"root_cause_category" binding:"required"`
	FixApplied        string            `json:"fix_applied" binding:"required,min=10"`
	PreventionSteps   string            `json:"prevention_steps" binding:"required,min=10"`
	SubmittedBy       string            `json:"submitted_by"`
}

func (r *SubmitRCARequest) Validate() error {
	if r.IncidentStart.IsZero() || r.IncidentEnd.IsZero() {
		return ErrIncompleteRCA
	}
	if r.IncidentEnd.Before(r.IncidentStart) {
		return ErrInvalidTimeRange
	}
	if r.FixApplied == "" || r.PreventionSteps == "" || string(r.RootCauseCategory) == "" {
		return ErrIncompleteRCA
	}
	return nil
}
