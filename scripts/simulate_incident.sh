#!/usr/bin/env bash
# Simulates a cascading RDBMS outage and recovery workflow.
# Usage: ./scripts/simulate_incident.sh [BASE_URL]

set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
API="$BASE_URL/api/v1"
BOLD='\033[1m'; RED='\033[0;31m'; YELLOW='\033[0;33m'; GREEN='\033[0;32m'; BLUE='\033[0;34m'; NC='\033[0m'

log()   { echo -e "${BOLD}[$(date +%H:%M:%S)]${NC} $*"; }
ok()    { echo -e "${GREEN}  ✓ $*${NC}"; }
info()  { echo -e "${BLUE}  → $*${NC}"; }
warn()  { echo -e "${YELLOW}  ⚠ $*${NC}"; }
error() { echo -e "${RED}  ✗ $*${NC}"; }

post_signal() {
  local payload="$1"
  local resp
  resp=$(curl -s -w '\n%{http_code}' -X POST "$API/signals" \
    -H 'Content-Type: application/json' -d "$payload")
  local code; code=$(echo "$resp" | tail -n 1)
  local body; body=$(echo "$resp" | sed '$d')
  if [[ "$code" == "202" ]]; then
    ok "Signal queued (id=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4 | head -1))"
  else
    error "HTTP $code: $body"
  fi
}

echo
echo -e "${BOLD}═══════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}  IMS – Incident Simulation: RDBMS Cascading Outage    ${NC}"
echo -e "${BOLD}═══════════════════════════════════════════════════════${NC}"
echo

# ── Phase 1: RDBMS failure (P0) ────────────────────────────────────
log "Phase 1: RDBMS Primary Failure (expect P0 work item)"

for i in $(seq 1 5); do
  post_signal '{
    "component_id":   "POSTGRES_PRIMARY_01",
    "component_type": "RDBMS",
    "error_code":     "CONN_REFUSED",
    "message":        "Connection refused to primary PostgreSQL node. Health check failed.",
    "severity":       "CRITICAL",
    "metadata":       {"host": "db-primary-01.internal", "attempt": '"$i"'}
  }'
  sleep 0.2
done
info "5 RDBMS signals sent → debouncer should create 1 P0 work item"

sleep 1

# ── Phase 2: Cascade to MCP_HOST ───────────────────────────────────
log "Phase 2: MCP Host degradation (expect P1 work item)"

for i in $(seq 1 3); do
  post_signal '{
    "component_id":   "MCP_HOST_CLUSTER_A",
    "component_type": "MCP_HOST",
    "error_code":     "UPSTREAM_TIMEOUT",
    "message":        "MCP Host cannot reach RDBMS upstream. Requests timing out.",
    "severity":       "HIGH",
    "metadata":       {"upstream": "POSTGRES_PRIMARY_01", "timeout_ms": 5000}
  }'
  sleep 0.2
done

# ── Phase 3: Cache pressure ─────────────────────────────────────────
log "Phase 3: Cache eviction spike (expect P2 work item)"

post_signal '{
  "component_id":   "CACHE_CLUSTER_01",
  "component_type": "CACHE",
  "error_code":     "EVICTION_SPIKE",
  "message":        "Cache eviction rate spiked 400% as DB-fallback load increases.",
  "severity":       "MEDIUM",
  "metadata":       {"eviction_rate": 4200, "memory_usage_pct": 94}
}'

# ── Phase 4: Burst test (debounce verification) ─────────────────────
log "Phase 4: 20-signal burst for POSTGRES_PRIMARY_01 (should link to existing work item)"

for i in $(seq 1 20); do
  post_signal '{
    "component_id":   "POSTGRES_PRIMARY_01",
    "component_type": "RDBMS",
    "error_code":     "CONN_REFUSED",
    "message":        "Continued connection failures to primary node.",
    "severity":       "CRITICAL",
    "metadata":       {"burst_seq": '"$i"'}
  }' &
done
wait
info "20 burst signals sent — verify only 1 work item for POSTGRES_PRIMARY_01 in dashboard"

sleep 2

# ── Phase 5: Query work items ───────────────────────────────────────
log "Phase 5: Listing active work items"
WI_RESP=$(curl -s "$API/work-items")
WI_COUNT=$(echo "$WI_RESP" | grep -o '"id"' | wc -l | tr -d ' ')
ok "Found $WI_COUNT work item(s)"
echo "$WI_RESP" | python3 -c "
import sys, json
data = json.load(sys.stdin).get('data', [])
for wi in data:
    print(f'  [{wi[\"priority\"]}] {wi[\"status\"]:15} {wi[\"component_id\"]:30} signals={wi[\"signal_count\"]}')
" 2>/dev/null || echo "$WI_RESP" | head -c 500

# ── Phase 6: State transition workflow ─────────────────────────────
log "Phase 6: Driving first work item through full lifecycle"

FIRST_ID=$(echo "$WI_RESP" | python3 -c "
import sys, json
data = json.load(sys.stdin).get('data', [])
if data: print(data[0]['id'])
" 2>/dev/null)

if [[ -z "$FIRST_ID" ]]; then
  warn "No work items found to transition — skipping lifecycle test"
else
  info "Work item: $FIRST_ID"

  info "Transitioning OPEN → INVESTIGATING"
  curl -s -X PATCH "$API/work-items/$FIRST_ID/status" \
    -H 'Content-Type: application/json' \
    -d '{"status":"INVESTIGATING"}' | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'  status={d[\"status\"]}')" 2>/dev/null

  sleep 1
  info "Transitioning INVESTIGATING → RESOLVED"
  curl -s -X PATCH "$API/work-items/$FIRST_ID/status" \
    -H 'Content-Type: application/json' \
    -d '{"status":"RESOLVED"}' | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'  status={d[\"status\"]}')" 2>/dev/null

  sleep 1
  info "Submitting RCA"
  NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  START=$(date -u -v-2H +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d '2 hours ago' +"%Y-%m-%dT%H:%M:%SZ")
  RCA_RESP=$(curl -s -X POST "$API/work-items/$FIRST_ID/rca" \
    -H 'Content-Type: application/json' \
    -d "{
      \"incident_start\":      \"$START\",
      \"incident_end\":        \"$NOW\",
      \"root_cause_category\": \"Infrastructure Failure\",
      \"fix_applied\":         \"Promoted replica to primary. Updated connection strings across all services. Verified replication health.\",
      \"prevention_steps\":    \"Implement automatic failover with Patroni. Add PagerDuty alert for replica lag > 10s. Monthly failover drills.\",
      \"submitted_by\":        \"simulation-script\"
    }")
  RCA_ID=$(echo "$RCA_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id','error'))" 2>/dev/null)
  ok "RCA created: $RCA_ID"

  sleep 1
  info "Closing incident (RESOLVED → CLOSED)"
  CLOSE_RESP=$(curl -s -X PATCH "$API/work-items/$FIRST_ID/status" \
    -H 'Content-Type: application/json' \
    -d '{"status":"CLOSED"}')
  MTTR=$(echo "$CLOSE_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('mttr_seconds','N/A'))" 2>/dev/null)
  ok "Incident CLOSED. MTTR = ${MTTR}s"
fi

# ── Phase 7: Verify RCA gate ───────────────────────────────────────
log "Phase 7: Verifying RCA gate (should reject CLOSED without RCA)"

GATE_ID=$(echo "$WI_RESP" | python3 -c "
import sys, json
data = json.load(sys.stdin).get('data', [])
resolved = [wi for wi in data if wi['status'] == 'RESOLVED']
if resolved: print(resolved[0]['id'])
" 2>/dev/null)

if [[ -n "$GATE_ID" ]]; then
  GATE_RESP=$(curl -s -o /dev/null -w '%{http_code}' -X PATCH "$API/work-items/$GATE_ID/status" \
    -H 'Content-Type: application/json' -d '{"status":"CLOSED"}')
  if [[ "$GATE_RESP" == "422" ]]; then
    ok "RCA gate working correctly (HTTP 422 received)"
  else
    warn "Expected 422, got $GATE_RESP"
  fi
fi

echo
echo -e "${BOLD}═══════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}${BOLD}  Simulation complete! Open the dashboard at:${NC}"
echo -e "${BLUE}  http://localhost:3000${NC}  (Docker)"
echo -e "${BLUE}  http://localhost:5173${NC}  (dev server)"
echo -e "${BOLD}═══════════════════════════════════════════════════════${NC}"
echo
