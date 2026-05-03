
# IMS – Mission-Critical Incident Management System

A production-grade distributed incident management platform that ingests high-throughput failure signals, deduplicates them into tracked Work Items, enforces a mandatory RCA-gated workflow, and provides a real-time React dashboard.

---

## Architecture Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                React Dashboard  (port 3001)                  │
│   Live Feed · Incident Detail · RCA Form · Metrics Bar       │
└───────────────────────────┬──────────────────────────────────┘
                            │  REST + WebSocket
┌───────────────────────────▼──────────────────────────────────┐
│                  Go Backend  (port 8080)                     │
│                                                              │
│  Rate Limiter → Signal Buffer (100K) → Worker Pool (×20)     │
│                                   ↓                          │
│                            Debouncer (sync.Map)              │
│                    ┌──────────┼──────────┐                   │
│               PostgreSQL   MongoDB     Redis                 │
│             (work items   (signals    (dashboard             │
│               + RCA)       audit log)  hot-path)             │
│                                                              │
│  State Machine (State pattern) · Alerter (Strategy pattern)  | 
│  WebSocket Hub · Metrics Goroutine (/5s stdout)              │
└──────────────────────────────────────────────────────────────┘
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22 + Gin |
| RDBMS | PostgreSQL 16 |
| NoSQL / Data Lake | MongoDB 7 |
| Cache / Hot-path | Redis 7 |
| Frontend | React 18 + TypeScript + Vite + Tailwind CSS |
| Containers | Docker Compose |

---

## Quick Start (Docker Compose)

```bash
git clone <repo>
cd ims
docker-compose up --build
```

| Service | URL |
|---------|-----|
| Dashboard | http://localhost:3001 |
| Backend API | http://localhost:8080 |
| Health check | http://localhost:8080/health |
| Metrics | http://localhost:8080/metrics |

---

## Local Development

### Backend
```bash
cd backend
cp .env.example .env   # edit if needed
go run ./cmd/server
```
Requires Postgres, MongoDB, Redis running locally (or use `docker-compose up postgres mongo redis`).

### Frontend
```bash
cd frontend
npm install
npm run dev            # http://localhost:5173
```
Vite proxies `/api` to `localhost:8080`.

---

## Run the Simulation

```bash
# With Docker running:
bash scripts/simulate_incident.sh

# Or against a different host:
bash scripts/simulate_incident.sh http://localhost:8080
```

The script:
1. Sends 5 RDBMS signals → creates 1 P0 work item (debounce demonstration)
2. Sends MCP_HOST and CACHE signals → creates P1 and P2 work items
3. Fires 20 burst signals at the same component concurrently
4. Drives the first work item through the full lifecycle: OPEN → INVESTIGATING → RESOLVED → RCA → CLOSED
5. Verifies the RCA gate rejects CLOSED without RCA (expects HTTP 422)

---

## API Reference

### Ingest Signal
```http
POST /api/v1/signals
Content-Type: application/json

{
  "component_id":   "POSTGRES_PRIMARY_01",
  "component_type": "RDBMS",
  "error_code":     "CONN_REFUSED",
  "message":        "Connection refused on port 5432",
  "severity":       "CRITICAL",
  "metadata":       { "host": "db-01.internal" }
}
```
Returns `202 Accepted` (async). Returns `503` if buffer is full.

**Component Types:** `RDBMS` · `API` · `MCP_HOST` · `CACHE` · `QUEUE` · `NOSQL`  
**Severity:** `CRITICAL` · `HIGH` · `MEDIUM` · `LOW`

### List Work Items
```http
GET /api/v1/work-items?status=OPEN
```
Served from Redis cache for active incidents. Falls back to PostgreSQL.

### Transition Status
```http
PATCH /api/v1/work-items/:id/status
{ "status": "INVESTIGATING" }
```
Valid transitions: `OPEN → INVESTIGATING → RESOLVED → CLOSED`  
CLOSED is rejected with 422 if no RCA is submitted.

### Submit RCA
```http
POST /api/v1/work-items/:id/rca
{
  "incident_start":      "2024-01-15T10:00:00Z",
  "incident_end":        "2024-01-15T12:30:00Z",
  "root_cause_category": "Infrastructure Failure",
  "fix_applied":         "Promoted replica to primary...",
  "prevention_steps":    "Implement Patroni auto-failover...",
  "submitted_by":        "engineer@company.com"
}
```
Only accepted when work item is in `RESOLVED` state.

### Get Raw Signals (Data Lake)
```http
GET /api/v1/work-items/:id/signals?limit=50&skip=0
```
Paginated signal history from MongoDB.

### WebSocket
```
ws://localhost:8080/api/v1/ws
```
Receives `{"type":"work_item_update","data":{...}}` on any work item change.

---

## How Backpressure Works

The system uses three tiers to stay alive at 10K signals/sec even with a slow database:

```
Tier 1 – Rate Limiter (token bucket, 2000 rps default)
  └── Malicious or runaway clients get 429 + Retry-After

Tier 2 – In-memory Channel Buffer (100,000 capacity)
  ├── HTTP handler pushes to channel and returns 202 immediately
  ├── 20 worker goroutines drain the channel at DB speed
  └── At 10K/sec peak → 10 seconds of headroom before hitting tier 3

Tier 3 – Buffer Full (503 Service Unavailable)
  ├── Returned only if DB is so slow that 100K buffer fills up
  └── Retry-After: 1 tells clients to back off
```

The API **never blocks** on DB operations. If MongoDB or Postgres are degraded, signals queue in RAM and are processed as soon as the DB recovers.

---

## Design Patterns

### State Pattern — Work Item Lifecycle
```
interface State {
    Status() WorkItemStatus
    CanTransitionTo(next) bool
    OnExit(next, rca) error        ← RCA validation lives here
}

OpenState → InvestigatingState → ResolvedState → ClosedState
                                       ↑
                              OnExit validates RCA completeness
                              before allowing CLOSED transition
```

### Strategy Pattern — Alert Routing
```
interface AlertStrategy {
    Alert(workItem)
    Priority() Priority
}

ComponentType  →  Strategy           →  Priority  →  Channel
RDBMS          →  rdbmsStrategy      →  P0        →  PagerDuty+SMS+Slack
API/MCP/Queue  →  p1Strategy         →  P1        →  Slack+Email
CACHE          →  p2Strategy         →  P2        →  Slack
NOSQL          →  p3Strategy         →  P3        →  Email
```

---

## MTTR Calculation

MTTR is calculated automatically from the RCA submission:

```
mttr_seconds = RCA.incident_end - RCA.incident_start
```

Stored on the `work_items` table and displayed in the dashboard. MTTR reflects the engineer-assessed incident window, not just the time to close the ticket.

---

## Testing the RCA Gate

```bash
# This must return HTTP 422:
curl -X PATCH http://localhost:8080/api/v1/work-items/{RESOLVED_ID}/status \
  -H 'Content-Type: application/json' \
  -d '{"status":"CLOSED"}'

# Response: {"error":"cannot close: RCA not submitted..."}
```

---

## Observability

- **Stdout metrics every 5s:** `[METRICS] ingested=X/s  processed=Y/s  dropped=Z  ...`
- **`GET /health`:** Checks Postgres, MongoDB, Redis connectivity + buffer fill
- **`GET /metrics`:** JSON snapshot of all counters

---

## Repository Structure

```
/
├── backend/
│   ├── cmd/server/main.go           Entry point
│   └── internal/
│       ├── config/                  Environment config
│       ├── models/                  Signal, WorkItem, RCA, errors
│       ├── storage/
│       │   ├── postgres/            Schema, work items, RCA, retry helper
│       │   ├── mongodb/             Signal insert, query, aggregation
│       │   └── redis/               Dashboard cache, sorted set
│       ├── ingestion/
│       │   ├── buffer.go            Buffered channel, backpressure
│       │   ├── debouncer.go         sync.Map component→workItem mapping
│       │   └── processor.go         Worker pool, orchestrates all stores
│       ├── workflow/
│       │   ├── state.go             State pattern (4 concrete states)
│       │   └── alerting.go          Strategy pattern (4 alert strategies)
│       ├── api/
│       │   ├── handlers.go          All HTTP handlers
│       │   ├── middleware.go        Rate limiter, request ID
│       │   ├── routes.go            Gin router setup
│       │   └── websocket.go         Hub + client pump goroutines
│       └── metrics/collector.go     Atomic counters, 5s printer
├── frontend/
│   └── src/
│       ├── components/              Dashboard, IncidentList, IncidentDetail,
│       │                           RCAForm, StatusBadge, MetricsBar
│       ├── hooks/useWebSocket.ts   Reconnecting WS hook
│       ├── api/client.ts           Typed fetch wrapper
│       └── types/index.ts          All TypeScript types
├── scripts/
│   ├── simulate_incident.sh        Full lifecycle simulation
│   └── mock_failure_event.json     Sample cascading failure payload
├── docs/
│   ├── architecture.md             Detailed ASCII diagrams + schema
│   └── system-design.md            Design decisions, concurrency notes
├── docker-compose.yml
├── PROMPTS.md                      All prompts/specs used (submission req)
└── README.md
```
