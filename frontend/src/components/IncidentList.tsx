import { formatDistanceToNow } from 'date-fns'
import type { WorkItem } from '../types'
import { StatusBadge, PriorityBadge } from './StatusBadge'

interface Props {
  items: WorkItem[]
  selectedId: string | null
  onSelect: (id: string) => void
}

export function IncidentList({ items, selectedId, onSelect }: Props) {
  if (items.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-gray-600 gap-2">
        <span className="text-3xl">✓</span>
        <p className="font-mono text-sm">No active incidents</p>
      </div>
    )
  }

  return (
    <div className="divide-y divide-border overflow-y-auto scrollbar-thin">
      {items.map(wi => (
        <button
          key={wi.id}
          onClick={() => onSelect(wi.id)}
          className={`w-full text-left px-4 py-3 hover:bg-white/5 transition-colors ${
            selectedId === wi.id ? 'bg-white/10 border-l-2 border-blue-500' : ''
          }`}
        >
          <div className="flex items-start justify-between gap-2 mb-1.5">
            <div className="flex items-center gap-2 min-w-0">
              <PriorityBadge priority={wi.priority} />
              <span className="text-xs font-mono text-gray-400 truncate">{wi.component_id}</span>
            </div>
            <StatusBadge status={wi.status} />
          </div>
          <p className="text-sm text-gray-200 truncate mb-1">{wi.title}</p>
          <div className="flex items-center gap-3 text-xs text-gray-500 font-mono">
            <span>{wi.signal_count.toLocaleString()} signals</span>
            <span>·</span>
            <span>{formatDistanceToNow(new Date(wi.start_time), { addSuffix: true })}</span>
            {wi.mttr_seconds && (
              <>
                <span>·</span>
                <span className="text-green-500">MTTR {formatMTTR(wi.mttr_seconds)}</span>
              </>
            )}
          </div>
        </button>
      ))}
    </div>
  )
}

function formatMTTR(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  return `${(seconds / 3600).toFixed(1)}h`
}
