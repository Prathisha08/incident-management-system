export type ComponentType = 'RDBMS' | 'API' | 'MCP_HOST' | 'CACHE' | 'QUEUE' | 'NOSQL'
export type Severity = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'
export type WorkItemStatus = 'OPEN' | 'INVESTIGATING' | 'RESOLVED' | 'CLOSED'
export type Priority = 'P0' | 'P1' | 'P2' | 'P3'
export type RootCauseCategory =
  | 'Infrastructure Failure'
  | 'Software Bug'
  | 'Configuration Error'
  | 'Network Issue'
  | 'Capacity / Scaling Issue'
  | 'Third-party Service Failure'
  | 'Human Error'
  | 'Unknown'

export interface Signal {
  id: string
  component_id: string
  component_type: ComponentType
  error_code: string
  message: string
  severity: Severity
  metadata?: Record<string, unknown>
  work_item_id?: string
  received_at: string
}

export interface RCA {
  id: string
  work_item_id: string
  incident_start: string
  incident_end: string
  root_cause_category: RootCauseCategory
  fix_applied: string
  prevention_steps: string
  submitted_by: string
  submitted_at: string
}

export interface WorkItem {
  id: string
  component_id: string
  component_type: ComponentType
  title: string
  status: WorkItemStatus
  priority: Priority
  signal_count: number
  start_time: string
  resolved_at?: string
  closed_at?: string
  mttr_seconds?: number
  rca?: RCA
  created_at: string
  updated_at: string
}

export interface SubmitRCAPayload {
  incident_start: string
  incident_end: string
  root_cause_category: RootCauseCategory
  fix_applied: string
  prevention_steps: string
  submitted_by?: string
}

export interface WSMessage {
  type: 'work_item_update' | 'ping'
  data?: Partial<WorkItem>
}

export interface MetricsSnapshot {
  received: number
  processed: number
  dropped: number
  buffer_len: number
  buffer_cap: number
}
