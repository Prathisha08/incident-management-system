package models

import "time"

type ComponentType string

const (
	ComponentRDBMS   ComponentType = "RDBMS"
	ComponentAPI     ComponentType = "API"
	ComponentMCPHost ComponentType = "MCP_HOST"
	ComponentCache   ComponentType = "CACHE"
	ComponentQueue   ComponentType = "QUEUE"
	ComponentNoSQL   ComponentType = "NOSQL"
)

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
)

type Signal struct {
	ID            string                 `json:"id" bson:"_id"`
	ComponentID   string                 `json:"component_id" bson:"component_id"`
	ComponentType ComponentType          `json:"component_type" bson:"component_type"`
	ErrorCode     string                 `json:"error_code" bson:"error_code"`
	Message       string                 `json:"message" bson:"message"`
	Severity      Severity               `json:"severity" bson:"severity"`
	Metadata      map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
	WorkItemID    string                 `json:"work_item_id,omitempty" bson:"work_item_id,omitempty"`
	ReceivedAt    time.Time              `json:"received_at" bson:"received_at"`
}

type IngestSignalRequest struct {
	ComponentID   string                 `json:"component_id" binding:"required"`
	ComponentType ComponentType          `json:"component_type" binding:"required"`
	ErrorCode     string                 `json:"error_code" binding:"required"`
	Message       string                 `json:"message" binding:"required"`
	Severity      Severity               `json:"severity" binding:"required"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
