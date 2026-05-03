package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/aravindhsrbk/ims/internal/models"
)

func (c *Client) InsertSignal(ctx context.Context, signal *models.Signal) error {
	return withRetry(ctx, func() error {
		_, err := c.signals().InsertOne(ctx, signal)
		return err
	})
}

func (c *Client) BulkInsertSignals(ctx context.Context, signals []*models.Signal) error {
	if len(signals) == 0 {
		return nil
	}
	docs := make([]interface{}, len(signals))
	for i, s := range signals {
		docs[i] = s
	}
	return withRetry(ctx, func() error {
		_, err := c.signals().InsertMany(ctx, docs)
		return err
	})
}

func (c *Client) GetSignalsByWorkItem(ctx context.Context, workItemID string, limit, skip int64) ([]*models.Signal, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "received_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(skip)

	cursor, err := c.signals().Find(ctx, bson.M{"work_item_id": workItemID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var signals []*models.Signal
	if err := cursor.All(ctx, &signals); err != nil {
		return nil, err
	}
	return signals, nil
}

func (c *Client) CountSignalsByWorkItem(ctx context.Context, workItemID string) (int64, error) {
	return c.signals().CountDocuments(ctx, bson.M{"work_item_id": workItemID})
}

func (c *Client) GetSignalTimeseries(ctx context.Context, componentID string, from, to time.Time) ([]bson.M, error) {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"component_id": componentID,
			"received_at":  bson.M{"$gte": from, "$lte": to},
		}},
		bson.M{"$group": bson.M{
			"_id": bson.M{
				"year":   bson.M{"$year": "$received_at"},
				"month":  bson.M{"$month": "$received_at"},
				"day":    bson.M{"$dayOfMonth": "$received_at"},
				"hour":   bson.M{"$hour": "$received_at"},
				"minute": bson.M{"$minute": "$received_at"},
			},
			"count": bson.M{"$sum": 1},
		}},
		bson.M{"$sort": bson.M{"_id": 1}},
	}
	cursor, err := c.signals().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func withRetry(ctx context.Context, fn func() error) error {
	delays := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	var lastErr error
	for _, delay := range delays {
		if lastErr = fn(); lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}
