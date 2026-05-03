package postgres

import (
	"context"

	"github.com/aravindhsrbk/ims/internal/models"
)

func (c *Client) CreateRCA(ctx context.Context, rca *models.RCA) error {
	return withRetry(ctx, func() error {
		_, err := c.pool.Exec(ctx, `
			INSERT INTO rca_records
				(id, work_item_id, incident_start, incident_end, root_cause_category, fix_applied, prevention_steps, submitted_by, submitted_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())`,
			rca.ID, rca.WorkItemID, rca.IncidentStart, rca.IncidentEnd,
			string(rca.RootCauseCategory), rca.FixApplied, rca.PreventionSteps, rca.SubmittedBy,
		)
		return err
	})
}

func (c *Client) GetRCAByWorkItem(ctx context.Context, workItemID string) (*models.RCA, error) {
	row := c.pool.QueryRow(ctx, `
		SELECT id, work_item_id, incident_start, incident_end, root_cause_category,
		       fix_applied, prevention_steps, submitted_by, submitted_at
		FROM rca_records WHERE work_item_id = $1`, workItemID)

	rca := &models.RCA{}
	err := row.Scan(
		&rca.ID, &rca.WorkItemID, &rca.IncidentStart, &rca.IncidentEnd,
		&rca.RootCauseCategory, &rca.FixApplied, &rca.PreventionSteps,
		&rca.SubmittedBy, &rca.SubmittedAt,
	)
	if err != nil {
		return nil, nil // no RCA yet
	}
	return rca, nil
}
