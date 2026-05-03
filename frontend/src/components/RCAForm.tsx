import { useState } from 'react'
import { api } from '../api/client'
import type { RootCauseCategory, SubmitRCAPayload } from '../types'

const ROOT_CAUSE_OPTIONS: RootCauseCategory[] = [
  'Infrastructure Failure',
  'Software Bug',
  'Configuration Error',
  'Network Issue',
  'Capacity / Scaling Issue',
  'Third-party Service Failure',
  'Human Error',
  'Unknown',
]

interface Props {
  workItemId: string
  onSuccess: () => void
}

export function RCAForm({ workItemId, onSuccess }: Props) {
  const [form, setForm] = useState<Partial<SubmitRCAPayload>>({
    root_cause_category: 'Unknown',
    submitted_by: '',
  })
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const set = (field: keyof SubmitRCAPayload) => (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>
  ) => setForm(f => ({ ...f, [field]: e.target.value }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (!form.incident_start || !form.incident_end) {
      setError('Incident start and end times are required')
      return
    }
    if (!form.fix_applied || form.fix_applied.length < 10) {
      setError('Fix applied must be at least 10 characters')
      return
    }
    if (!form.prevention_steps || form.prevention_steps.length < 10) {
      setError('Prevention steps must be at least 10 characters')
      return
    }

    setSubmitting(true)
    try {
      await api.submitRCA(workItemId, form as SubmitRCAPayload)
      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit RCA')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <Field label="Incident Start">
          <input
            type="datetime-local"
            required
            className={inputClass}
            value={form.incident_start ?? ''}
            onChange={set('incident_start')}
          />
        </Field>
        <Field label="Incident End">
          <input
            type="datetime-local"
            required
            className={inputClass}
            value={form.incident_end ?? ''}
            onChange={set('incident_end')}
          />
        </Field>
      </div>

      <Field label="Root Cause Category">
        <select
          className={inputClass}
          value={form.root_cause_category}
          onChange={set('root_cause_category')}
          required
        >
          {ROOT_CAUSE_OPTIONS.map(o => (
            <option key={o} value={o}>{o}</option>
          ))}
        </select>
      </Field>

      <Field label="Fix Applied" hint="Describe the exact fix that was applied (min 10 chars)">
        <textarea
          required
          minLength={10}
          rows={4}
          className={inputClass}
          placeholder="e.g. Restarted the primary RDBMS instance, increased connection pool size from 50 to 200..."
          value={form.fix_applied ?? ''}
          onChange={set('fix_applied')}
        />
      </Field>

      <Field label="Prevention Steps" hint="What will prevent this from recurring? (min 10 chars)">
        <textarea
          required
          minLength={10}
          rows={4}
          className={inputClass}
          placeholder="e.g. Add connection pool monitoring alerts, implement circuit breaker pattern..."
          value={form.prevention_steps ?? ''}
          onChange={set('prevention_steps')}
        />
      </Field>

      <Field label="Submitted By (optional)">
        <input
          type="text"
          className={inputClass}
          placeholder="engineer@company.com"
          value={form.submitted_by ?? ''}
          onChange={set('submitted_by')}
        />
      </Field>

      {error && (
        <div className="px-3 py-2 bg-red-500/10 border border-red-500/30 rounded text-red-400 text-sm">
          {error}
        </div>
      )}

      <button
        type="submit"
        disabled={submitting}
        className="w-full py-2.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-semibold rounded transition-colors"
      >
        {submitting ? 'Submitting...' : 'Submit RCA'}
      </button>
    </form>
  )
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-xs font-mono font-medium text-gray-400 mb-1.5 uppercase tracking-wide">
        {label}
      </label>
      {hint && <p className="text-xs text-gray-600 mb-1">{hint}</p>}
      {children}
    </div>
  )
}

const inputClass =
  'w-full bg-surface border border-border rounded px-3 py-2 text-sm text-gray-100 placeholder-gray-600 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/30 transition-colors'
