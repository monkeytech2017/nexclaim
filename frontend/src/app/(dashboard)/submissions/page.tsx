'use client'
import { useState } from 'react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { claimBatchesApi, hospitalsApi } from '@/lib/api'
import { useIdentity } from '@/lib/auth-context'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { FormatBadge, InsclBadge } from '@/components/ui/badges'
import { RefreshCw, AlertCircle, CheckCircle2, Clock, Activity } from 'lucide-react'

const FORMATS = ['', '16FILES', 'CIPN', 'CSOP', 'AIPN', 'SSOP']
const STATUSES = ['', 'pending', 'sent', 'error']

export default function SubmissionsPage() {
  const identity = useIdentity()
  const isHospital = identity?.role === 'hospital'
  const [hcode, setHcode] = useState(isHospital ? identity.hcode ?? '' : '')
  const [period, setPeriod] = useState('')
  const [inscl, setInscl] = useState('')
  const [format, setFormat] = useState('')
  const [status, setStatus] = useState('')

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })
  const list = useQuery({
    queryKey: ['claim-batches', hcode, period, inscl, format, status],
    queryFn: () => claimBatchesApi.list({
      hcode: hcode || undefined, period: period || undefined, inscl: inscl || undefined,
      format: format || undefined, status: status || undefined,
    }),
    refetchInterval: 30_000,
  })

  if (list.isLoading) return <LoadingBlock />
  if (list.isError) return <ErrorBlock error={list.error} onRetry={() => list.refetch()} />

  const rows = list.data?.items ?? []
  const hcodes = hospitals.data?.hospitals.map(h => h.hcode) ?? []

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        ทุกครั้งที่ pipeline submit (ไม่ใช่ dry-run) จะได้ row ใน claim_batch พร้อม zip/md5/txnId.
        Fetch REP แล้ว c-code จะ link กลับมาที่ row นี้.
      </p>

      <div className="flex items-center gap-2 flex-wrap">
        <select value={hcode} onChange={e => setHcode(e.target.value)}
          disabled={isHospital}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1 disabled:bg-gray-50 disabled:text-gray-500">
          <option value="">ทั้ง รพ.</option>
          {hcodes.map(h => <option key={h} value={h}>{h}</option>)}
          {isHospital && hcode && !hcodes.includes(hcode) && <option value={hcode}>{hcode}</option>}
        </select>
        <input type="text" value={period} onChange={e => setPeriod(e.target.value)}
          placeholder="YYYYMM" maxLength={6}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1 font-mono w-24" />
        <input type="text" value={inscl} onChange={e => setInscl(e.target.value.toUpperCase())}
          placeholder="INSCL"
          className="text-xs border border-gray-200 rounded-lg px-2 py-1 font-mono w-20" />
        <select value={format} onChange={e => setFormat(e.target.value)}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1">
          {FORMATS.map(f => <option key={f} value={f}>{f || 'ทุก format'}</option>)}
        </select>
        <select value={status} onChange={e => setStatus(e.target.value)}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1">
          {STATUSES.map(s => <option key={s} value={s}>{s || 'ทุกสถานะ'}</option>)}
        </select>
        <span className="text-xs text-gray-400">{rows.length} รายการ</span>
        <button onClick={() => list.refetch()}
          className="ml-auto text-xs px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-600 inline-flex items-center gap-1.5">
          <RefreshCw className={`w-3.5 h-3.5 ${list.isFetching ? 'animate-spin' : ''}`} /> รีเฟรช
        </button>
      </div>

      {rows.length === 0 ? (
        <EmptyBlock label="ยังไม่มี submission — ส่ง claim สักครั้งก่อน (OPD Batches / IPD Imports / ส่ง Claim)" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">เวลา</th>
                <th className="px-4 py-2.5 text-left font-medium">HCODE/Period</th>
                <th className="px-4 py-2.5 text-left font-medium">INSCL</th>
                <th className="px-4 py-2.5 text-left font-medium">Format</th>
                <th className="px-4 py-2.5 text-left font-medium">Sender</th>
                <th className="px-4 py-2.5 text-right font-medium">Records</th>
                <th className="px-4 py-2.5 text-left font-medium">Status</th>
                <th className="px-4 py-2.5 text-left font-medium">TxnID</th>
                <th className="px-4 py-2.5 text-left font-medium">C-code</th>
                <th className="px-4 py-2.5 text-left font-medium">ZIP</th>
                <th className="px-4 py-2.5 text-right font-medium">Log</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(b => (
                <tr key={b.batch_id} className="hover:bg-gray-50">
                  <td className="px-4 py-2 text-xs text-gray-500">
                    {formatDT(b.created_at)}
                    {b.sent_at && <div className="text-[10px] text-gray-400">sent {formatDT(b.sent_at)}</div>}
                  </td>
                  <td className="px-4 py-2 font-mono text-xs">{b.hcode} / {b.period}</td>
                  <td className="px-4 py-2"><InsclBadge inscl={b.inscl} /></td>
                  <td className="px-4 py-2"><FormatBadge format={b.format as any} /></td>
                  <td className="px-4 py-2 font-mono text-[11px] text-gray-500">{b.sender}</td>
                  <td className="px-4 py-2 text-right text-xs">
                    <span className="text-gray-700">{b.total_records}</span>
                    {b.error_records > 0 && <span className="ml-1 text-amber-700">({b.error_records}✗)</span>}
                  </td>
                  <td className="px-4 py-2"><StatusPill status={b.status} /></td>
                  <td className="px-4 py-2 font-mono text-[11px] text-gray-700 max-w-[140px] truncate" title={b.fdh_txn_id}>
                    {b.fdh_txn_id || '—'}
                  </td>
                  <td className="px-4 py-2 text-xs">
                    {b.c_code_count === 0 ? (
                      <span className="text-gray-300">—</span>
                    ) : (
                      <Link href={`/c-codes?batch=${encodeURIComponent(b.batch_id)}`}
                        className="inline-flex items-center gap-1 text-primary-600 hover:underline">
                        <AlertCircle className="w-3 h-3" />
                        {b.c_code_open}/{b.c_code_count} open
                      </Link>
                    )}
                  </td>
                  <td className="px-4 py-2 font-mono text-[11px] text-gray-500 max-w-[160px] truncate" title={b.zip_filename}>
                    {b.zip_filename || '—'}
                    {b.zip_md5 && <div className="text-[10px] text-gray-400 truncate" title={b.zip_md5}>{b.zip_md5.slice(0, 12)}…</div>}
                  </td>
                  <td className="px-4 py-2 text-right">
                    <Link href={`/send-logs?batch=${encodeURIComponent(b.batch_id)}`}
                      className="inline-flex items-center gap-1 text-[11px] text-gray-500 hover:text-primary-600">
                      <Activity className="w-3 h-3" /> log
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {rows.some(r => r.error_msg) && (
        <details className="bg-white rounded-xl border border-gray-100 p-3">
          <summary className="text-xs text-gray-600 cursor-pointer hover:text-gray-900">Error messages ({rows.filter(r => r.error_msg).length})</summary>
          <ul className="mt-2 text-xs space-y-1">
            {rows.filter(r => r.error_msg).map(r => (
              <li key={r.batch_id} className="text-gray-700">
                <code className="bg-gray-100 px-1 rounded mr-2">{r.hcode}/{r.period}/{r.inscl}/{r.format}</code>
                <span className="text-red-700">{r.error_msg}</span>
              </li>
            ))}
          </ul>
        </details>
      )}
    </div>
  )
}

function StatusPill({ status }: { status: string }) {
  const map: Record<string, { label: string; cls: string; icon?: React.ComponentType<{ className?: string }> }> = {
    sent:    { label: 'sent',    cls: 'bg-green-50 text-green-700', icon: CheckCircle2 },
    pending: { label: 'pending', cls: 'bg-gray-100 text-gray-600',  icon: Clock },
    error:   { label: 'error',   cls: 'bg-red-50 text-red-700',     icon: AlertCircle },
  }
  const s = map[status] ?? { label: status, cls: 'bg-gray-100 text-gray-600' }
  const Icon = s.icon
  return (
    <span className={`inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full font-medium ${s.cls}`}>
      {Icon && <Icon className="w-3 h-3" />}{s.label}
    </span>
  )
}

function formatDT(iso: string): string {
  try {
    return new Date(iso).toLocaleString('th-TH', { hour12: false, dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return iso
  }
}
