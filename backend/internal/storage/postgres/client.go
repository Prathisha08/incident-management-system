package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	pool *pgxpool.Pool
}

func NewClient(dsn string) (*Client, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	c := &Client{pool: pool}
	if err := c.migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("postgres: migrate: %w", err)
	}
	return c, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

func (c *Client) Close() {
	c.pool.Close()
}

func (c *Client) migrate(ctx context.Context) error {
	_, err := c.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS work_items (
			id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			component_id   VARCHAR(255)  NOT NULL,
			component_type VARCHAR(50)   NOT NULL,
			title          TEXT          NOT NULL,
			status         VARCHAR(50)   NOT NULL DEFAULT 'OPEN',
			priority       VARCHAR(10)   NOT NULL,
			signal_count   BIGINT        NOT NULL DEFAULT 1,
			start_time     TIMESTAMPTZ   NOT NULL,
			resolved_at    TIMESTAMPTZ,
			closed_at      TIMESTAMPTZ,
			mttr_seconds   BIGINT,
			created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
			updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS rca_records (
			id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			work_item_id        UUID NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
			incident_start      TIMESTAMPTZ   NOT NULL,
			incident_end        TIMESTAMPTZ   NOT NULL,
			root_cause_category VARCHAR(100)  NOT NULL,
			fix_applied         TEXT          NOT NULL,
			prevention_steps    TEXT          NOT NULL,
			submitted_by        VARCHAR(255)  NOT NULL DEFAULT 'system',
			submitted_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
			UNIQUE(work_item_id)
		);

		CREATE INDEX IF NOT EXISTS idx_work_items_status        ON work_items(status);
		CREATE INDEX IF NOT EXISTS idx_work_items_component_id  ON work_items(component_id);
		CREATE INDEX IF NOT EXISTS idx_work_items_priority_status ON work_items(priority, status);
	`)
	return err
}
