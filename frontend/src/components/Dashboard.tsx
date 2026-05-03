import { useState, useCallback, useEffect } from 'react'
import { api } from '../api/client'
import { useWebSocket } from '../hooks/useWebSocket'
import type { WorkItem, WSMessage, WorkItemStatus } from '../types'
import { IncidentList } from './IncidentList'
import { IncidentDetail } from './IncidentDetail'
import { MetricsBar } from './MetricsBar'

const STATUS_FILTERS: { label: string; value: string }[] = [
  { label: 'Active', value: '' },
  { label: 'Open', value: 'OPEN' },
  { label: 'Investigating', value: 'INVESTIGATING' },
  { label: 'Resolved', value: 'RESOLVED' },
  { label: 'Closed', value: 'CLOSED' },
  { label: 'All', value: 'ALL' },
]

export function Dashboard() {
  const [workItems, setWorkItems] = useState<WorkItem[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState('')
  const [loading, setLoading] = useState(true)
  const [lastUpdate, setLastUpdate] = useState<Date>(new Date())

  const loadWorkItems = useCallback(async () => {
    try {
      const res = await api.listWorkItems(statusFilter || undefined)
      setWorkItems(res.data ?? [])
    } catch (err) {
      console.error('Failed to load work items:', err)
    } finally {
      setLoading(false)
    }
  }, [statusFilter])

  useEffect(() => {
    setLoading(true)
    loadWorkItems()
  }, [loadWorkItems])

  // Re-fetch every 30s as a safety net even without WS updates
  useEffect(() => {
    const id = setInterval(loadWorkItems, 30_000)
    return () => clearInterval(id)
  }, [loadWorkItems])

  const handleWSMessage = useCallback((msg: WSMessage) => {
    if (msg.type !== 'work_item_update' || !msg.data) return
    setLastUpdate(new Date())

    const update = msg.data
    if (!update) return
    setWorkItems(prev => {
      const idx = prev.findIndex(wi => wi.id === update.id)
      if (idx === -1) {
        loadWorkItems()
        return prev
      }
      const updated = [...prev]
      updated[idx] = { ...updated[idx], ...update } as WorkItem

      if (
        (update.status as WorkItemStatus) === 'CLOSED' &&
        (statusFilter === '' || statusFilter === 'ACTIVE')
      ) {
        return updated.filter(wi => wi.status !== 'CLOSED')
      }
      return updated
    })
  }, [loadWorkItems, statusFilter])

  useWebSocket(handleWSMessage)

  const activeCount = workItems.filter(wi => wi.status !== 'CLOSED').length
  const p0Count = workItems.filter(wi => wi.priority === 'P0' && wi.status !== 'CLOSED').length

  return (
    <div className="flex flex-col h-screen bg-surface">
      {/* Top bar */}
      <header className="flex items-center justify-between px-5 py-3 bg-panel border-b border-border">
        <div className="flex items-center gap-3">
          <div className="w-2 h-2 rounded-full bg-blue-500" />
          <span className="font-mono font-bold text-sm tracking-widest text-gray-200">IMS</span>
          <span className="text-gray-600 text-xs font-mono">Incident Management System</span>
        </div>
        <div className="flex items-center gap-4 text-xs font-mono">
          {p0Count > 0 && (
            <span className="flex items-center gap-1.5 px-2 py-1 bg-red-500/20 border border-red-500/40 rounded text-red-400 font-semibold animate-pulse">
              ⚠ {p0Count} P0
            </span>
          )}
          <span className="text-gray-500">
            {activeCount} active · updated {lastUpdate.toLocaleTimeString()}
          </span>
        </div>
      </header>

      <MetricsBar />

      <div className="flex flex-1 overflow-hidden">
        {/* Left panel: incident list */}
        <div className="w-[380px] shrink-0 flex flex-col border-r border-border">
          {/* Filter tabs */}
          <div className="flex overflow-x-auto border-b border-border px-2">
            {STATUS_FILTERS.map(f => (
              <button
                key={f.value}
                onClick={() => { setStatusFilter(f.value); setSelectedId(null) }}
                className={`px-3 py-2 text-xs font-mono whitespace-nowrap transition-colors border-b-2 ${
                  statusFilter === f.value
                    ? 'border-blue-500 text-blue-400'
                    : 'border-transparent text-gray-500 hover:text-gray-300'
                }`}
              >
                {f.label}
              </button>
            ))}
          </div>

          <div className="flex-1 overflow-y-auto scrollbar-thin">
            {loading ? (
              <div className="flex items-center justify-center h-32 text-gray-600 text-xs font-mono">
                Loading incidents...
              </div>
            ) : (
              <IncidentList
                items={workItems}
                selectedId={selectedId}
                onSelect={setSelectedId}
              />
            )}
          </div>
        </div>

        {/* Right panel: incident detail */}
        <div className="flex-1 overflow-hidden">
          {selectedId ? (
            <IncidentDetail
              key={selectedId}
              workItemId={selectedId}
              onClose={() => setSelectedId(null)}
              onStatusChange={loadWorkItems}
            />
          ) : (
            <EmptyState />
          )}
        </div>
      </div>
    </div>
  )
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center h-full text-gray-700 gap-3">
      <div className="w-16 h-16 rounded-full border-2 border-gray-700 flex items-center justify-center text-2xl">
        ↑
      </div>
      <p className="font-mono text-sm">Select an incident to view details</p>
      <p className="text-xs text-gray-600">Raw signals, state machine, and RCA form</p>
    </div>
  )
}
