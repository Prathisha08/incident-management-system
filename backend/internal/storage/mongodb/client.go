package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Client struct {
	client   *mongo.Client
	database string
}

func NewClient(uri, database string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Client().ApplyURI(uri)
	c, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("mongodb: connect: %w", err)
	}
	if err := c.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongodb: ping: %w", err)
	}

	client := &Client{client: c, database: database}
	if err := client.createIndexes(ctx); err != nil {
		return nil, fmt.Errorf("mongodb: indexes: %w", err)
	}
	return client, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, nil)
}

func (c *Client) Close() {
	_ = c.client.Disconnect(context.Background())
}

func (c *Client) signals() *mongo.Collection {
	return c.client.Database(c.database).Collection("signals")
}

func (c *Client) createIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "component_id", Value: 1}, {Key: "received_at", Value: -1}}},
		{Keys: bson.D{{Key: "work_item_id", Value: 1}}},
		{Keys: bson.D{{Key: "received_at", Value: -1}}},
	}
	_, err := c.signals().Indexes().CreateMany(ctx, indexes)
	return err
}
