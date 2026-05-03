import { useState, useEffect, useCallback } from 'react'
import { format } from 'date-fns'
import { api } from '../api/client'
import type { WorkItem, Signal, WorkItemStatus } from '../types'
import { StatusBadge, PriorityBadge, SeverityText } from './StatusBadge'
import { RCAForm } from './RCAForm'

const NEXT_STATUS: Partial<Record<WorkItemStatus, { label: string; status: WorkItemStatus; classes: string }>> = {
  OPEN:          { label: 'Start Investigating', status: 'INVESTIGATING', classes: 'bg-yellow-600 hover:bg-yellow-500' },
  INVESTIGATING: { label: 'Mark Resolved',       status: 'RESOLVED',      classes: 'bg-blue-600 hover:bg-blue-500' },
  RESOLVED:      { label: 'Close Incident',      status: 'CLOSED',        classes: 'bg-gray-600 hover:bg-gray-500' },
}

interface Props {
  workItemId: string
  onClose: () => void
  onStatusChange: () => void
}

export function IncidentDetail({ workItemId, onClose, onStatusChange }: Props) {
  const [wi, setWi] = useState<WorkItem | null>(null)
  const [signals, setSignals] = useState<Signal[]>([])
  const [totalSignals, setTotalSignals] = useState(0)
  const [signalPage, setSignalPage] = useState(0)
  const [tab, setTab] = useState<'signals' | 'rca'>('signals')
  const [transitioning, setTransitioning] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const PAGE_SIZE = 20

  const load = useCallback(async () => {
    try {
      const [wiData, sigData] = await Promise.all([
        api.getWorkItem(workItemId),
        api.getSignals(workItemId, PAGE_SIZE, signalPage * PAGE_SIZE),
      ])
      setWi(wiData)
      setSignals(sigData.data)
      setTotalSignals(sigData.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load')
    }
  }, [workItemId, signalPage])

  useEffect(() => { load() }, [load])

  const transition = async () => {
    if (!wi) return
    const next = NEXT_STATUS[wi.status]
    if (!next) return

    if (next.status === 'CLOSED' && !wi.rca) {
      setTab('rca')
      setError('Submit RCA first, then close the incident')
      return
    }
    setError(null)
    setTransitioning(true)
    try {
      const updated = await api.transitionStatus(wi.id, next.status)
      setWi(updated)
      onStatusChange()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Transition failed')
    } finally {
      setTransitioning(false)
    }
  }

  const onRCASuccess = () => {
    load()
    setTab('signals')
  }

  if (!wi) {
    return (
      <div className="flex items-center justify-center h-full text-gray-600 font-mono text-sm">
        {error ?? 'Loading...'}
      </div>
    )
  }

  const nextAction = NEXT_STATUS[wi.status]

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Header */}
      <div className="px-5 py-4 border-b border-border flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-2 mb-2 flex-wrap">
            <PriorityBadge priority={wi.priority} />
            <StatusBadge status={wi.status} />
            <span className="text-xs font-mono text-gray-500">{wi.component_type}</span>
          </div>
          <h2 className="text-sm font-semibold text-gray-100 leading-snug">{wi.title}</h2>
          <p className="text-xs font-mono text-gray-500 mt-1">
            {wi.component_id} · {wi.signal_count.toLocaleString()} signals ·
            started {format(new Date(wi.start_time), 'MMM d, HH:mm:ss')}
            {wi.mttr_seconds != null && ` · MTTR ${formatMTTR(wi.mttr_seconds)}`}
          </p>
        </div>
        <button
          onClick={onClose}
          className="shrink-0 text-gray-500 hover:text-gray-200 transition-colors text-xl leading-none"
        >
          ×
        </button>
      </div>

      {/* Transition button */}
      {nextAction && wi.status !== 'CLOSED' && (
        <div className="px-5 py-2 border-b border-border flex items-center gap-3">
          <button
            onClick={transition}
            disabled={transitioning}
            className={`px-3 py-1.5 rounded text-xs font-semibold text-white transition-colors disabled:opacity-50 ${nextAction.classes}`}
          >
            {transitioning ? 'Processing...' : nextAction.label}
          </button>
          {error && <span className="text-xs text-red-400">{error}</span>}
        </div>
      )}

      {/* Tabs */}
      <div className="flex border-b border-border text-xs font-mono">
        <TabBtn active={tab === 'signals'} onClick={() => setTab('signals')}>
          Raw Signals ({totalSignals})
        </TabBtn>
        <TabBtn active={tab === 'rca'} onClick={() => setTab('rca')}>
          RCA {wi.rca ? '✓' : ''}
        </TabBtn>
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-y-auto scrollbar-thin">
        {tab === 'signals' && (
          <div>
            {signals.map(sig => (
              <div key={sig.id} className="px-4 py-2.5 border-b border-border/50 hover:bg-white/5 transition-colors">
                <div className="flex items-center justify-between mb-1 gap-2">
                  <span className="font-mono text-xs text-gray-400">{sig.error_code}</span>
                  <div className="flex items-center gap-2">
                    <SeverityText severity={sig.severity} />
                    <span className="text-xs text-gray-600 font-mono">
                      {format(new Date(sig.received_at), 'HH:mm:ss.SSS')}
                    </span>
                  </div>
                </div>
                <p className="text-xs text-gray-300">{sig.message}</p>
                {sig.metadata && Object.keys(sig.metadata).length > 0 && (
                  <details className="mt-1">
                    <summary className="text-xs text-gray-600 cursor-pointer hover:text-gray-400">metadata</summary>
                    <pre className="text-xs text-gray-500 mt-1 bg-surface rounded p-2 overflow-x-auto">
                      {JSON.stringify(sig.metadata, null, 2)}
                    </pre>
                  </details>
                )}
              </div>
            ))}
            {totalSignals > PAGE_SIZE && (
              <div className="flex items-center justify-between px-4 py-2 text-xs text-gray-500 font-mono">
                <button
                  disabled={signalPage === 0}
                  onClick={() => setSignalPage(p => p - 1)}
                  className="disabled:opacity-40 hover:text-gray-200 transition-colors"
                >
                  ← prev
                </button>
                <span>page {signalPage + 1} / {Math.ceil(totalSignals / PAGE_SIZE)}</span>
                <button
                  disabled={(signalPage + 1) * PAGE_SIZE >= totalSignals}
                  onClick={() => setSignalPage(p => p + 1)}
                  className="disabled:opacity-40 hover:text-gray-200 transition-colors"
                >
                  next →
                </button>
              </div>
            )}
          </div>
        )}

        {tab === 'rca' && (
          <div className="p-5">
            {wi.rca ? (
              <RCADisplay rca={wi.rca} />
            ) : wi.status === 'RESOLVED' ? (
              <RCAForm workItemId={wi.id} onSuccess={onRCASuccess} />
            ) : (
              <div className="text-sm text-gray-500 text-center py-8">
                RCA can be submitted once the incident is in <strong className="text-blue-400">RESOLVED</strong> state.
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function TabBtn({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className={`px-4 py-2.5 border-b-2 transition-colors ${
        active
          ? 'border-blue-500 text-blue-400'
          : 'border-transparent text-gray-500 hover:text-gray-300'
      }`}
    >
      {children}
    </button>
  )
}

function RCADisplay({ rca }: { rca: NonNullable<WorkItem['rca']> }) {
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <InfoField label="Incident Start" value={format(new Date(rca.incident_start), 'MMM d yyyy, HH:mm')} />
        <InfoField label="Incident End" value={format(new Date(rca.incident_end), 'MMM d yyyy, HH:mm')} />
      </div>
      <InfoField label="Root Cause Category" value={rca.root_cause_category} />
      <InfoField label="Fix Applied" value={rca.fix_applied} multiline />
      <InfoField label="Prevention Steps" value={rca.prevention_steps} multiline />
      <div className="text-xs font-mono text-gray-600">
        Submitted by {rca.submitted_by} · {format(new Date(rca.submitted_at), 'MMM d yyyy HH:mm')}
      </div>
    </div>
  )
}

function InfoField({ label, value, multiline }: { label: string; value: string; multiline?: boolean }) {
  return (
    <div>
      <p className="text-xs font-mono text-gray-500 uppercase tracking-wide mb-1">{label}</p>
      {multiline
        ? <p className="text-sm text-gray-200 whitespace-pre-wrap">{value}</p>
        : <p className="text-sm font-medium text-gray-200">{value}</p>
      }
    </div>
  )
}

function formatMTTR(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  return `${(seconds / 3600).toFixed(1)}h`
}
