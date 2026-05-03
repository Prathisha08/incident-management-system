package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/aravindhsrbk/ims/internal/models"
)

const (
	keyWorkItemPrefix = "wi:"
	keyDashboard      = "dashboard:active"
	ttlWorkItem       = 24 * time.Hour
	ttlDashboard      = 5 * time.Minute
)

func (c *Client) CacheWorkItem(ctx context.Context, wi *models.WorkItem) error {
	data, err := json.Marshal(wi)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s%s", keyWorkItemPrefix, wi.ID)

	pipe := c.rdb.Pipeline()
	pipe.Set(ctx, key, data, ttlWorkItem)

	priorityScore := priorityScore(wi.Priority)
	if wi.Status != models.StatusClosed {
		pipe.ZAdd(ctx, keyDashboard, goredis.Z{Score: priorityScore, Member: wi.ID})
	} else {
		pipe.ZRem(ctx, keyDashboard, wi.ID)
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (c *Client) GetWorkItem(ctx context.Context, id string) (*models.WorkItem, error) {
	data, err := c.rdb.Get(ctx, fmt.Sprintf("%s%s", keyWorkItemPrefix, id)).Bytes()
	if err != nil {
		return nil, err
	}
	wi := &models.WorkItem{}
	return wi, json.Unmarshal(data, wi)
}

func (c *Client) GetDashboard(ctx context.Context) ([]*models.WorkItem, error) {
	ids, err := c.rdb.ZRangeByScore(ctx, keyDashboard, &goredis.ZRangeBy{
		Min: "-inf",
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}

	items := make([]*models.WorkItem, 0, len(ids))
	for _, id := range ids {
		wi, err := c.GetWorkItem(ctx, id)
		if err != nil {
			continue // stale entry
		}
		items = append(items, wi)
	}
	return items, nil
}

func (c *Client) InvalidateDashboard(ctx context.Context) error {
	return c.rdb.Del(ctx, keyDashboard).Err()
}

func priorityScore(p models.Priority) float64 {
	switch p {
	case models.PriorityP0:
		return 0
	case models.PriorityP1:
		return 1
	case models.PriorityP2:
		return 2
	default:
		return 3
	}
}
