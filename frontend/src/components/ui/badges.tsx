import type { BatchState, ClaimFormat } from '@/lib/api'

export function BatchStateBadge({ state }: { state: BatchState }) {
  const map: Record<BatchState, { label: string; cls: string }> = {
    RECEIVED:  { label: 'รับแล้ว',     cls: 'bg-blue-50 text-blue-700' },
    FETCHING:  { label: 'กำลังดึง',    cls: 'bg-amber-50 text-amber-700' },
    COMPLETED: { label: 'สำเร็จ',      cls: 'bg-green-50 text-green-700' },
    FAILED:    { label: 'ผิดพลาด',    cls: 'bg-red-50 text-red-700' },
  }
  const s = map[state] ?? { label: state, cls: 'bg-gray-100 text-gray-600' }
  return (
    <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${s.cls}`}>
      {s.label}
    </span>
  )
}

export function FormatBadge({ format }: { format: ClaimFormat }) {
  const cls: Record<ClaimFormat, string> = {
    '16FILES': 'bg-purple-50 text-purple-700',
    'CIPN':    'bg-blue-50 text-blue-700',
    'CSOP':    'bg-sky-50 text-sky-700',
    'AIPN':    'bg-indigo-50 text-indigo-700',
    'SSOP':    'bg-violet-50 text-violet-700',
  }
  return (
    <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-mono font-medium ${cls[format] ?? 'bg-gray-100 text-gray-600'}`}>
      {format}
    </span>
  )
}

export function InsclBadge({ inscl }: { inscl: string }) {
  return (
    <code className="text-xs bg-gray-100 text-gray-700 px-2 py-0.5 rounded font-mono">
      {inscl}
    </code>
  )
}
