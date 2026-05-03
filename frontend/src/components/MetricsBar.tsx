import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { MetricsSnapshot } from '../types'

export function MetricsBar() {
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null)
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    const refresh = async () => {
      try {
        const m = await api.metrics()
        setMetrics(m)
        const h = await api.health()
        setConnected(h.status === 'ok')
      } catch {
        setConnected(false)
      }
    }
    refresh()
    const id = setInterval(refresh, 5000)
    return () => clearInterval(id)
  }, [])

  const bufferPct = metrics ? Math.round((metrics.buffer_len / metrics.buffer_cap) * 100) : 0

  return (
    <div className="flex items-center gap-6 px-4 py-2 bg-panel border-b border-border text-xs font-mono">
      <span className={`flex items-center gap-1.5 font-semibold ${connected ? 'text-green-400' : 'text-red-400'}`}>
        <span className={`w-2 h-2 rounded-full ${connected ? 'bg-green-400' : 'bg-red-400'} animate-pulse`} />
        {connected ? 'SYSTEM ONLINE' : 'DEGRADED'}
      </span>
      {metrics && (
        <>
          <Stat label="RECEIVED" value={metrics.received.toLocaleString()} />
          <Stat label="PROCESSED" value={metrics.processed.toLocaleString()} />
          <Stat label="DROPPED" value={metrics.dropped.toLocaleString()} color={metrics.dropped > 0 ? 'text-red-400' : undefined} />
          <div className="flex items-center gap-2">
            <span className="text-gray-500">BUFFER</span>
            <div className="w-24 h-1.5 bg-gray-700 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all ${bufferPct > 80 ? 'bg-red-500' : bufferPct > 50 ? 'bg-yellow-500' : 'bg-green-500'}`}
                style={{ width: `${bufferPct}%` }}
              />
            </div>
            <span className="text-gray-400">{bufferPct}%</span>
          </div>
        </>
      )}
      <span className="ml-auto text-gray-600">{new Date().toLocaleTimeString()}</span>
    </div>
  )
}

function Stat({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <span className="text-gray-500">{label}</span>
      <span className={color ?? 'text-gray-200'}>{value}</span>
    </span>
  )
}
