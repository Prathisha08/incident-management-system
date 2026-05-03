# IMS Architecture

## System Overview

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         React Frontend (port 3000)                        │
│  ┌──────────────┐  ┌─────────────────────┐  ┌───────────────────────┐   │
│  │  Live Feed   │  │   Incident Detail   │  │      RCA Form         │   │
│  │ (WS updates) │  │  (signals, status)  │  │  (date, dropdown,     │   │
│  └──────────────┘  └─────────────────────┘  │   textareas)          │   │
│                                              └───────────────────────┘   │
└──────────────────────────────┬───────────────────────────────────────────┘
                               │ HTTP REST  /  WebSocket (ws://...)
┌──────────────────────────────▼───────────────────────────────────────────┐
│                          Go Backend (port 8080)                           │
│                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  Gin HTTP Router                                                    │ │
│  │  ├── POST /api/v1/signals           (rate-limited: 2000 rps)        │ │
│  │  ├── GET  /api/v1/work-items                                        │ │
│  │  ├── GET  /api/v1/work-items/:id                                    │ │
│  │  ├── PATCH /api/v1/work-items/:id/status                            │ │
│  │  ├── POST /api/v1/work-items/:id/rca                                │ │
│  │  ├── GET  /api/v1/work-items/:id/signals                            │ │
│  │  ├── GET  /api/v1/ws               (WebSocket upgrade)              │ │
│  │  ├── GET  /health                                                   │ │
│  │  └── GET  /metrics                                                  │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                                                           │
│  ┌──────────────────┐    ┌───────────────────────────────────────────┐  │
│  │  Rate Limiter     │    │         WebSocket Hub                     │  │
│  │  (token bucket)   │    │  goroutine: fan-out broadcast to clients  │  │
│  └────────┬─────────┘    └───────────────────────────────────────────┘  │
│           │                             ▲                                │
│  ┌────────▼───────────────────────┐     │                                │
│  │   Signal Buffer                │     │                                │
│  │   chan *Signal (cap=100,000)   │     │                                │
│  │   Non-blocking push → 202      │     │                                │
│  │   Full buffer     → 503        │     │                                │
│  └────────┬───────────────────────┘     │                                │
│           │                             │                                │
│  ┌────────▼─────────────────────────────┴──────────────────────────────┐ │
│  │                Worker Pool (20 goroutines)                           │ │
│  │                                                                      │ │
│  │  for each signal:                                                    │ │
│  │    1. Debouncer.GetOrCreate(componentID)                             │ │
│  │       └── sync.Map: componentID → {workItemID, status, lastSeen}    │ │
│  │    2. Write signal → MongoDB (async, 3x retry)                      │ │
│  │    3. If new: INSERT work_item → PostgreSQL (tx, 3x retry)          │ │
│  │    4. ZINCRBY / SET → Redis cache                                   │ │
│  │    5. AlerterRegistry.Alert(workItem)  [Strategy pattern]           │ │
│  │    6. Hub.Broadcast(JSON payload)                                   │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  Metrics Goroutine: prints signals/sec to stdout every 5s            │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
└───────────────────────┬──────────────────────────────────────────────────┘
                        │
          ┌─────────────┼──────────────────┐
          ▼             ▼                  ▼
  ┌───────────────┐ ┌───────────┐ ┌──────────────────┐
  │  PostgreSQL   │ │  MongoDB  │ │      Redis        │
  │               │ │           │ │                   │
  │  work_items   │ │  signals  │ │  wi:{id} → JSON  │
  │  rca_records  │ │  (audit   │ │  dashboard:active │
  │               │ │   log)    │ │  (sorted set by   │
  │  Transactional│ │  Indexed  │ │   priority score) │
  │  Source of    │ │  by comp  │ │                   │
  │  Truth        │ │  & WI ID  │ │  Hot-path cache   │
  └───────────────┘ └───────────┘ └──────────────────┘
```

## Design Patterns

### State Pattern — Work Item Lifecycle

```
 ┌──────┐    acknowledge    ┌───────────────┐    fix applied    ┌──────────┐
 │ OPEN │ ──────────────── ▶│ INVESTIGATING │ ──────────────── ▶│ RESOLVED │
 └──────┘                   └───────────────┘                   └─────┬────┘
                                                                       │
                                                    RCA validated       │
                                                    ──────────────────▶│
                                                                  ┌────▼──┐
                                                                  │CLOSED │
                                                                  └───────┘
```

Each state implements `CanTransitionTo()` and `OnExit()`. The `resolvedState.OnExit()` enforces the mandatory RCA check before CLOSED can be reached.

### Strategy Pattern — Alert Routing

| Component Type | Priority | Channel         |
|---------------|----------|-----------------|
| RDBMS         | P0       | PagerDuty+SMS+Slack |
| API           | P1       | Slack+Email     |
| MCP_HOST      | P1       | Slack+Email     |
| QUEUE         | P1       | Slack+Email     |
| CACHE         | P2       | Slack           |
| NOSQL         | P3       | Email           |

`AlerterRegistry` maps `ComponentType → AlertStrategy`. New component types can be registered at runtime without changing core logic.

## Data Flow

### Signal Ingestion (Happy Path)
```
Client → POST /signals → Rate Limiter (token bucket)
       → Validate JSON  → Push to buffered channel → 202 Accepted

Worker → Read from channel → Debouncer.GetOrCreate()
       → Insert signal to MongoDB (retry ×3)
       → If new WI: INSERT to PostgreSQL (retry ×3)
       → Increment signal_count if existing WI
       → ZADD to Redis sorted set (priority score)
       → Alert via Strategy
       → Broadcast via WebSocket hub
```

### Backpressure
```
Normal:  channel fill < 20%  → 202 Accepted, processing in <100ms
Spike:   channel fill 20–80% → 202 Accepted, workers catch up
Burst:   channel fill > 80%  → warns via metrics
Full:    channel.Push() fails → 503 + Retry-After: 1
```

## Database Schema

### PostgreSQL
```sql
work_items (
  id UUID PK, component_id, component_type, title,
  status, priority, signal_count,
  start_time, resolved_at, closed_at, mttr_seconds,
  created_at, updated_at
)

rca_records (
  id UUID PK, work_item_id FK→work_items,
  incident_start, incident_end, root_cause_category,
  fix_applied, prevention_steps, submitted_by, submitted_at,
  UNIQUE(work_item_id)   ← one RCA per incident
)
```

### MongoDB Collection: `signals`
```json
{
  "_id": "uuid",
  "component_id": "POSTGRES_PRIMARY_01",
  "component_type": "RDBMS",
  "error_code": "CONN_REFUSED",
  "message": "...",
  "severity": "CRITICAL",
  "metadata": { "...": "..." },
  "work_item_id": "uuid",
  "received_at": "2024-01-15T10:30:00Z"
}
```
Indexes: `{component_id, received_at}`, `{work_item_id}`, `{received_at}`

### Redis Key Space
```
wi:{uuid}          → JSON work item blob (TTL 24h)
dashboard:active   → Sorted set: member=workItemID, score=priority (0=P0)
```
