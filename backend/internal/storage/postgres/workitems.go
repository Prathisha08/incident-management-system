package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/aravindhsrbk/ims/internal/models"
)

func (c *Client) CreateWorkItem(ctx context.Context, wi *models.WorkItem) error {
	return withRetry(ctx, func() error {
		_, err := c.pool.Exec(ctx, `
			INSERT INTO work_items
				(id, component_id, component_type, title, status, priority, signal_count, start_time, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())`,
			wi.ID, wi.ComponentID, string(wi.ComponentType), wi.Title,
			string(wi.Status), string(wi.Priority), wi.SignalCount, wi.StartTime,
		)
		return err
	})
}

func (c *Client) IncrementSignalCount(ctx context.Context, workItemID string) error {
	return withRetry(ctx, func() error {
		_, err := c.pool.Exec(ctx,
			`UPDATE work_items SET signal_count = signal_count + 1, updated_at = NOW() WHERE id = $1`,
			workItemID,
		)
		return err
	})
}

func (c *Client) UpdateWorkItemStatus(ctx context.Context, workItemID string, status models.WorkItemStatus, mttr *int64) error {
	return withRetry(ctx, func() error {
		now := time.Now()
		switch status {
		case models.StatusResolved:
			_, err := c.pool.Exec(ctx,
				`UPDATE work_items SET status=$1, resolved_at=$2, updated_at=NOW() WHERE id=$3`,
				string(status), now, workItemID,
			)
			return err
		case models.StatusClosed:
			_, err := c.pool.Exec(ctx,
				`UPDATE work_items SET status=$1, closed_at=$2, mttr_seconds=$3, updated_at=NOW() WHERE id=$4`,
				string(status), now, mttr, workItemID,
			)
			return err
		default:
			_, err := c.pool.Exec(ctx,
				`UPDATE work_items SET status=$1, updated_at=NOW() WHERE id=$2`,
				string(status), workItemID,
			)
			return err
		}
	})
}

func (c *Client) GetWorkItem(ctx context.Context, id string) (*models.WorkItem, error) {
	row := c.pool.QueryRow(ctx, `
		SELECT id, component_id, component_type, title, status, priority,
		       signal_count, start_time, resolved_at, closed_at, mttr_seconds, created_at, updated_at
		FROM work_items WHERE id = $1`, id)

	wi := &models.WorkItem{}
	err := row.Scan(
		&wi.ID, &wi.ComponentID, &wi.ComponentType, &wi.Title, &wi.Status, &wi.Priority,
		&wi.SignalCount, &wi.StartTime, &wi.ResolvedAt, &wi.ClosedAt, &wi.MTTRSeconds,
		&wi.CreatedAt, &wi.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("work item not found: %w", err)
	}
	return wi, nil
}

func (c *Client) ListWorkItems(ctx context.Context, statusFilter string) ([]*models.WorkItem, error) {
	query := `
		SELECT id, component_id, component_type, title, status, priority,
		       signal_count, start_time, resolved_at, closed_at, mttr_seconds, created_at, updated_at
		FROM work_items`
	args := []interface{}{}

	if statusFilter != "" && statusFilter != "ALL" {
		query += ` WHERE status = $1`
		args = append(args, statusFilter)
	}
	query += ` ORDER BY
		CASE priority WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 WHEN 'P2' THEN 2 ELSE 3 END,
		created_at DESC`

	rows, err := c.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.WorkItem
	for rows.Next() {
		wi := &models.WorkItem{}
		if err := rows.Scan(
			&wi.ID, &wi.ComponentID, &wi.ComponentType, &wi.Title, &wi.Status, &wi.Priority,
			&wi.SignalCount, &wi.StartTime, &wi.ResolvedAt, &wi.ClosedAt, &wi.MTTRSeconds,
			&wi.CreatedAt, &wi.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, wi)
	}
	return items, rows.Err()
}
