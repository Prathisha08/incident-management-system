# System Design Notes

## Tech Stack Rationale

| Concern | Choice | Why |
|---------|--------|-----|
| Backend language | Go | Goroutines + channels = natural fit for high-throughput async signal processing. Single binary, minimal memory footprint. |
| HTTP framework | Gin | Low latency, minimal overhead, excellent middleware ecosystem. |
| WebSocket | gorilla/websocket | Battle-tested, full-duplex, used in production at scale. |
| RDBMS | PostgreSQL | ACID transactions for work item state, referential integrity for RCA, rich query support. |
| NoSQL | MongoDB | Flexible schema for raw signal payloads, native JSON, time-series aggregation pipeline, horizontal shale. |
| Cache | Redis | Sub-millisecond reads for dashboard hot-path; sorted sets for priority ordering; pub/sub available if needed. |
| Frontend | React + TypeScript + Vite | Fastest dev iteration, rich ecosystem, strong typing. |
| Styling | Tailwind CSS | Utility-first, no runtime CSS overhead, dark ops-dashboard theme. |

## Concurrency Design

### Worker Pool
- Fixed pool of 20 goroutines (configurable via `WORKER_COUNT`)
- Each goroutine is a long-lived `for { select }` loop over the signal channel
- No goroutine per request — prevents goroutine explosion under load
- Context cancellation propagates cleanly through all goroutines on shutdown

### Lock-free Metrics
- `sync/atomic` int64 counters for received/processed/dropped signals
- No mutex contention in the hot path
- Snapshot taken every 5 seconds by a dedicated goroutine

### WebSocket Hub
- Single hub goroutine owns the client map — no mutex needed for writes
- `fanOut()` uses a read-lock snapshot to avoid blocking under message storms
- Slow consumers are evicted (non-blocking channel send with `default` case)

### Debouncer
- `sync.Map` provides lock-striping for concurrent componentID lookups
- Per-entry `sync.Mutex` for CAS-style operations within an entry
- Eviction on CLOSED prevents the map from growing unbounded

## Backpressure Strategy

The system uses a three-tier backpressure model:

```
Tier 1 – Rate Limiter (token bucket, before buffer)
  ├── Prevents single clients from monopolising capacity
  └── Returns 429 with Retry-After header

Tier 2 – In-memory Channel Buffer (100,000 signals)
  ├── Absorbs burst up to 100K signals before any DB call is made
  ├── At 10K/s peak, this provides ~10 seconds of burst headroom
  └── Workers drain at the rate the DB allows

Tier 3 – Buffer Full Response (503 Service Unavailable)
  ├── Emitted only when buffer is completely full
  ├── Retry-After: 1 header signals clients to back off
  └── Dropped count tracked via atomic counter + /metrics
```

This ensures the API never blocks waiting for a slow DB and cannot OOM: if the DB is offline, the buffer fills and starts returning 503, which is the correct signal for clients to apply backpressure.

## Mandatory RCA Gate

The state machine enforces RCA before CLOSED:

1. `PATCH /work-items/:id/status` with `{"status":"CLOSED"}` first fetches the RCA from Postgres.
2. If no RCA exists → 422 Unprocessable with a clear error message.
3. If RCA exists → passes through `StateMachine.Transition()` which calls `resolvedState.OnExit()`.
4. `OnExit()` validates all fields: non-empty `fix_applied`, `prevention_steps`, `root_cause_category`, valid time range.
5. Only on full validation does the DB update occur.

MTTR is calculated as `incident_end - incident_start` from the RCA submission and stored in `work_items.mttr_seconds`.

## Debounce Window Logic

The debouncer uses a compound condition for "same work item":
- `e.workItemID != ""` — a work item exists
- `e.status ∈ {OPEN, INVESTIGATING}` — it's still active (not RESOLVED/CLOSED)
- `time.Since(e.lastSeen) < 100s` — (10× the 10s window) signals still being received

The 10× multiplier means an active incident stays linked as long as signals keep coming, but a component that goes quiet for >100s will get a fresh work item on the next failure. This is intentional: a brief recovery followed by a new outage should be a separate incident.

## Retry Logic

All DB writes use exponential backoff:
```
Attempt 1: immediate
Attempt 2: wait 100ms
Attempt 3: wait 200ms
Attempt 4: wait 400ms
Failure:   log error, increment dropped counter, continue
```

Context cancellation is checked between retries, ensuring clean shutdown even with a stuck DB.

## RCA Root Cause Categories

These align with common incident taxonomies (SRE handbook, ITIL):
- Infrastructure Failure
- Software Bug
- Configuration Error
- Network Issue
- Capacity / Scaling Issue
- Third-party Service Failure
- Human Error
- Unknown

## Observability

| Signal | Mechanism |
|--------|-----------|
| Throughput (signals/sec) | Atomic counters printed every 5s |
| Storage health | `/health` checks Postgres ping, MongoDB ping, Redis ping |
| Buffer pressure | `/metrics` exposes buffer_len / buffer_cap ratio |
| WebSocket clients | Hub tracks active connections |
| Drop rate | `dropped` counter in `/metrics` |
