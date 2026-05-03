# Prompts & Specs Used to Build This Repository

This file documents the prompts, specifications, and reasoning used to generate the IMS codebase, as required by the submission guidelines.

---

## Initial Challenge Prompt

The full engineering challenge specification was received verbatim. Key requirements extracted:

1. High-throughput signal ingestion (10K/sec) with in-memory backpressure
2. Debounce: 100 signals for same component in 10s → 1 work item
3. Storage: MongoDB (audit log), PostgreSQL (source of truth), Redis (hot-path cache)
4. Design patterns: State (lifecycle), Strategy (alerting)
5. Mandatory RCA before CLOSED transition
6. MTTR auto-calculation
7. React dashboard: live feed, incident detail, RCA form
8. Resilience: retry logic, rate limiting, /health, /metrics

---

## Architecture Planning Prompt

```
Build a Mission-Critical Incident Management System from scratch. 

Tech Stack decision:
- Backend: Go (goroutines/channels = modern concurrency primitives, single binary, 
  best fit for 10K signals/sec)
- HTTP: Gin (low overhead, excellent middleware)
- WebSocket: gorilla/websocket (production-grade, full-duplex)
- RDBMS: PostgreSQL (ACID, transactional work items + RCA)
- NoSQL: MongoDB (flexible schema, raw signal audit log, aggregation pipeline)
- Cache: Redis (sorted sets for priority ordering, sub-ms dashboard reads)
- Frontend: React 18 + TypeScript + Vite + Tailwind CSS
- Containers: Docker Compose

Core design decisions:
1. Signal Buffer: buffered Go channel (cap=100,000) decouples HTTP from persistence
2. Worker Pool: 20 goroutines drain the channel (configurable)
3. Debouncer: sync.Map for lock-free componentID → workItemID mapping
4. State Machine: State pattern with interface per lifecycle phase
5. Alerting: Strategy pattern with AlerterRegistry (factory)
6. WebSocket Hub: single goroutine owns client map, non-blocking fan-out
7. Retry: exponential backoff (100/200/400ms) on all DB writes
8. Rate Limiter: token bucket via golang.org/x/time/rate
```

---

## State Machine Design

```
Design a State pattern for Work Item lifecycle:
- States: OPEN, INVESTIGATING, RESOLVED, CLOSED
- Each state implements: Status(), CanTransitionTo(), OnExit()
- resolvedState.OnExit() enforces mandatory RCA validation before CLOSED
- StateMachine.Transition() returns MTTR (seconds) when closing
- Invalid transitions return descriptive errors
```

## Strategy Pattern Design

```
Design a Strategy pattern for component-type alerting:
- AlertStrategy interface: Alert(WorkItem), Priority()
- Implementations: rdbmsStrategy (P0), p1Strategy (API/MCP/Queue), 
  p2Strategy (Cache), p3Strategy (NoSQL)
- AlerterRegistry: map[ComponentType]AlertStrategy (factory/registry)
- Console output simulates PagerDuty/Slack/Email channels
- Register() allows runtime extension
```

## Backpressure Design

```
The system must handle 10K signals/sec without crashing if persistence is slow.

Solution: Three-tier backpressure
1. Rate limiter (token bucket) → 429 for abusive clients
2. In-memory channel buffer (100K capacity) → absorbs 10s burst at peak
3. Buffer-full check → 503 + Retry-After: 1 for genuine overload

Workers drain the buffer at whatever rate the DB allows.
If DB is slow: buffer fills, 503s increase, clients back off.
No goroutine created per request, no blocking on DB calls in handler.
```

## Debounce Logic

```
When 100 signals arrive for CACHE_CLUSTER_01 in 10 seconds:
1. First signal: Debouncer.GetOrCreate() → no entry → calls createFn() → creates PostgreSQL work item → stores {workItemID, status=OPEN, lastSeen=now}
2. Signals 2–100: entry exists, status=OPEN, lastSeen recent → return existing workItemID, isNew=false
3. All 100 signals linked to same work item in MongoDB
4. PostgreSQL signal_count incremented 99 times (2–100)
5. Only 1 work item created, 100 signals stored
```

## Frontend Architecture

```
React 18 + TypeScript + Tailwind dark ops-dashboard theme.

Components:
- Dashboard: top-level state, WS connection, filter tabs
- IncidentList: sorted by priority (P0 first), click to select
- IncidentDetail: signals tab (paginated, from MongoDB) + RCA tab
- RCAForm: date pickers, root cause dropdown, textareas, validation
- MetricsBar: live throughput metrics, buffer fill, system status
- StatusBadge / PriorityBadge: colored semantic badges

WebSocket: reconnecting hook (3s backoff), fan-out updates to Dashboard state
API client: typed fetch wrapper, all endpoints, error propagation
```

---

## Key Engineering Decisions

| Decision | Rationale |
|----------|-----------|
| Go over Node.js | Native goroutines/channels > event loop + worker_threads for this workload |
| Buffered channel over Redis queue | Lower latency, no external dependency for in-flight signals, sufficient for 10s burst |
| sync.Map for debouncer | Lock-striping > global mutex at high concurrency |
| Gin over net/http | Built-in middleware, routing, validation — saves 200+ LoC |
| PostgreSQL schema migration on startup | Simplest deployment, idempotent, no migration tool dependency |
| MongoDB sorted by received_at DESC | Natural pagination for signal audit log |
| Redis sorted sets for dashboard | O(log N) priority ordering, instant reads, auto-eviction on CLOSED |
| 100K channel buffer | At 10K/sec peak, provides 10s headroom — enough for any realistic DB hiccup |
| 20 worker goroutines | Enough to saturate Postgres connection pool without overwhelming it |
