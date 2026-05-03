import type { WorkItemStatus, Priority, Severity } from '../types'

const statusConfig: Record<WorkItemStatus, { label: string; classes: string }> = {
  OPEN:          { label: 'OPEN',          classes: 'bg-red-500/20 text-red-400 border-red-500/40' },
  INVESTIGATING: { label: 'INVESTIGATING', classes: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/40' },
  RESOLVED:      { label: 'RESOLVED',      classes: 'bg-blue-500/20 text-blue-400 border-blue-500/40' },
  CLOSED:        { label: 'CLOSED',        classes: 'bg-gray-500/20 text-gray-400 border-gray-500/40' },
}

const priorityConfig: Record<Priority, { classes: string }> = {
  P0: { classes: 'bg-red-600 text-white' },
  P1: { classes: 'bg-orange-500 text-white' },
  P2: { classes: 'bg-yellow-500 text-black' },
  P3: { classes: 'bg-gray-500 text-white' },
}

const severityConfig: Record<Severity, { classes: string }> = {
  CRITICAL: { classes: 'text-red-400' },
  HIGH:     { classes: 'text-orange-400' },
  MEDIUM:   { classes: 'text-yellow-400' },
  LOW:      { classes: 'text-green-400' },
}

export function StatusBadge({ status }: { status: WorkItemStatus }) {
  const cfg = statusConfig[status]
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded border text-xs font-mono font-medium ${cfg.classes}`}>
      {cfg.label}
    </span>
  )
}

export function PriorityBadge({ priority }: { priority: Priority }) {
  const cfg = priorityConfig[priority]
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-mono font-bold ${cfg.classes}`}>
      {priority}
    </span>
  )
}

export function SeverityText({ severity }: { severity: Severity }) {
  const cfg = severityConfig[severity]
  return <span className={`text-xs font-mono font-medium ${cfg.classes}`}>{severity}</span>
}
