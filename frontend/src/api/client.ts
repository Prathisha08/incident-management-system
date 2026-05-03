import type { WorkItem, Signal, RCA, SubmitRCAPayload, MetricsSnapshot, WorkItemStatus } from '../types'

const BASE = '/api/v1'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error ?? `HTTP ${res.status}`)
  }
  return res.json()
}

export const api = {
  listWorkItems: (status?: string) =>
    request<{ data: WorkItem[]; source: string }>(
      `/work-items${status ? `?status=${status}` : ''}`
    ),

  getWorkItem: (id: string) => request<WorkItem>(`/work-items/${id}`),

  transitionStatus: (id: string, status: WorkItemStatus) =>
    request<WorkItem>(`/work-items/${id}/status`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    }),

  getSignals: (id: string, limit = 50, skip = 0) =>
    request<{ data: Signal[]; total: number; limit: number; skip: number }>(
      `/work-items/${id}/signals?limit=${limit}&skip=${skip}`
    ),

  submitRCA: (id: string, payload: SubmitRCAPayload) =>
    request<RCA>(`/work-items/${id}/rca`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  ingestSignal: (payload: object) =>
    request<{ id: string; status: string }>('/signals', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  health: () => fetch('/health').then(r => r.json()),

  metrics: () => fetch('/metrics').then(r => r.json() as Promise<MetricsSnapshot>),
}
