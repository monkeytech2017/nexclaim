'use client'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { auditApi, hospitalsApi, type AuditEntry } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { RoleBadge } from '@/components/ui/badges'

type TargetKindFilter = '' | 'api_key' | 'ccode' | 'rep' | 'claim_batch'

export default function AuditLogPage() {
  const [action, setAction] = useState('')
  const [hcode, setHcode] = useState('')
  const [targetKind, setTargetKind] = useState<TargetKindFilter>('')
  const [limit, setLimit] = useState(200)

  const list = useQuery({
    queryKey: ['audit-log', action, hcode, targetKind, limit],
    queryFn: () =>
      auditApi.list({
        action:      action      || undefined,
        hcode:       hcode       || undefined,
        target_kind: targetKind  || undefined,
        limit,
      }),
    refetchInterval: 30_000,
  })

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })
  const hospitalOptions = hospitals.data?.hospitals ?? []

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        Audit log — บันทึก event ที่ผู้ดูแลระบบทำ (admin-only, append-only).
      </p>

      <div className="flex items-center gap-2 flex-wrap bg-white border border-gray-100 rounded-xl px-3 py-2">
        <input
          type="text"
          value={action}
          onChange={e => setAction(e.target.value)}
          placeholder="action (เช่น api_key.create)"
          className="text-xs border border-gray-200 rounded-lg px-2 py-1 w-60 font-mono"
        />
        <select
          value={hcode}
          onChange={e => setHcode(e.target.value)}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1 font-mono"
        >
          <option value="">ทุก HCODE</option>
          {hospitalOptions.map(h => (
            <option key={h.hcode} value={h.hcode}>
              {h.hcode}
            </option>
          ))}
        </select>
        <select
          value={targetKind}
          onChange={e => setTargetKind(e.target.value as TargetKindFilter)}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1"
        >
          <option value="">ทุก target</option>
          <option value="api_key">api_key</option>
          <option value="ccode">ccode</option>
          <option value="rep">rep</option>
          <option value="claim_batch">claim_batch</option>
        </select>
        <select
          value={limit}
          onChange={e => setLimit(Number(e.target.value))}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1"
        >
          <option value={100}>100 รายการ</option>
          <option value={200}>200 รายการ</option>
          <option value={500}>500 รายการ</option>
        </select>
        <button
          onClick={() => list.refetch()}
          className="text-xs px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-600 inline-flex items-center gap-1.5"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${list.isFetching ? 'animate-spin' : ''}`} /> รีเฟรช
        </button>
        <span className="text-xs text-gray-400 ml-auto">
          {list.data?.items?.length ?? 0} รายการ
        </span>
      </div>

      {list.isLoading ? (
        <LoadingBlock />
      ) : list.isError ? (
        <ErrorBlock error={list.error} onRetry={() => list.refetch()} />
      ) : (list.data?.items ?? []).length === 0 ? (
        <EmptyBlock label="ไม่มี event ในช่วงเวลาที่เลือก" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">เวลา</th>
                <th className="px-4 py-2.5 text-left font-medium">Actor</th>
                <th className="px-4 py-2.5 text-left font-medium">Action</th>
                <th className="px-4 py-2.5 text-left font-medium">Target</th>
                <th className="px-4 py-2.5 text-left font-medium">HCODE</th>
                <th className="px-4 py-2.5 text-left font-medium">Payload</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {(list.data?.items ?? []).map(e => (
                <AuditRow key={e.id} entry={e} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function AuditRow({ entry }: { entry: AuditEntry }) {
  const payloadFull = entry.payload ? JSON.stringify(entry.payload) : ''
  const payloadShort = payloadFull.length > 80 ? payloadFull.slice(0, 80) + '…' : payloadFull
  const target = entry.target_kind
    ? `${entry.target_kind}${entry.target_id ? ':' + entry.target_id : ''}`
    : '—'

  return (
    <tr className="hover:bg-gray-50">
      <td className="px-4 py-2.5 text-xs text-gray-500 whitespace-nowrap">{formatDT(entry.created_at)}</td>
      <td className="px-4 py-2.5 text-xs">
        <div className="flex items-center gap-2">
          <span className="text-gray-900">{entry.actor_name || '—'}</span>
          {entry.actor_role && entry.actor_role !== 'system' && (
            <RoleBadge role={entry.actor_role as 'admin' | 'hospital'} />
          )}
          {entry.actor_role === 'system' && (
            <span className="text-[11px] bg-gray-100 text-gray-600 px-2 py-0.5 rounded-full font-medium">
              system
            </span>
          )}
        </div>
      </td>
      <td className="px-4 py-2.5">
        <code className="text-xs bg-gray-100 text-gray-800 px-2 py-0.5 rounded font-mono">
          {entry.action}
        </code>
      </td>
      <td className="px-4 py-2.5 text-xs font-mono text-gray-700 break-all">{target}</td>
      <td className="px-4 py-2.5 text-xs font-mono text-gray-700">{entry.hcode || '—'}</td>
      <td className="px-4 py-2.5 text-xs text-gray-500 font-mono">
        {payloadShort ? (
          <span title={payloadFull} className="break-all">
            {payloadShort}
          </span>
        ) : (
          <span className="text-gray-300">—</span>
        )}
      </td>
    </tr>
  )
}

function formatDT(iso: string): string {
  try {
    return new Date(iso).toLocaleString('th-TH', {
      hour12:     false,
      dateStyle:  'short',
      timeStyle:  'medium',
    })
  } catch {
    return iso
  }
}
