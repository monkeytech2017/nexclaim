'use client'
import { useState, Suspense } from 'react'
import { useSearchParams, useRouter } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { sendLogsApi, hospitalsApi } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { FormatBadge, InsclBadge } from '@/components/ui/badges'
import { RefreshCw, CheckCircle2, AlertCircle, X } from 'lucide-react'

export default function SendLogsPage() {
  return (
    <Suspense fallback={<LoadingBlock />}>
      <SendLogsContent />
    </Suspense>
  )
}

function SendLogsContent() {
  const searchParams = useSearchParams()
  const router = useRouter()
  const batchFromURL = searchParams.get('batch') ?? ''

  const [hcode, setHcode] = useState('')
  const [period, setPeriod] = useState('')
  const [success, setSuccess] = useState<'' | 'true' | 'false'>('')

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })
  const list = useQuery({
    queryKey: ['send-logs', batchFromURL, hcode, period, success],
    queryFn: () => sendLogsApi.list({
      batch_id: batchFromURL || undefined,
      hcode:    hcode  || undefined,
      period:   period || undefined,
      success:  success || undefined,
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
        Audit trail ทุกครั้งที่ pipeline ส่ง ZIP ไป FDH/CHI — สำเร็จหรือไม่สำเร็จก็บันทึก.
        ใช้ดู network latency, error body, และ re-attempt history.
      </p>

      {batchFromURL && (
        <div className="bg-blue-50 border border-blue-100 rounded-lg px-3 py-2 text-xs text-blue-900 flex items-center gap-2">
          <span>filtering to batch:</span>
          <code className="font-mono bg-white px-1.5 py-0.5 rounded border border-blue-200">{batchFromURL}</code>
          <button onClick={() => router.push('/send-logs')}
            className="ml-auto inline-flex items-center gap-1 text-blue-700 hover:text-blue-900">
            <X className="w-3 h-3" /> clear
          </button>
        </div>
      )}

      <div className="flex items-center gap-2 flex-wrap">
        <select value={hcode} onChange={e => setHcode(e.target.value)}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1">
          <option value="">ทั้ง รพ.</option>
          {hcodes.map(h => <option key={h} value={h}>{h}</option>)}
        </select>
        <input type="text" value={period} onChange={e => setPeriod(e.target.value)}
          placeholder="YYYYMM" maxLength={6}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1 font-mono w-24" />
        <select value={success} onChange={e => setSuccess(e.target.value as '' | 'true' | 'false')}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1">
          <option value="">ทั้งหมด</option>
          <option value="true">สำเร็จ</option>
          <option value="false">ล้มเหลว</option>
        </select>
        <span className="text-xs text-gray-400">{rows.length} รายการ</span>
        <button onClick={() => list.refetch()}
          className="ml-auto text-xs px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-600 inline-flex items-center gap-1.5">
          <RefreshCw className={`w-3.5 h-3.5 ${list.isFetching ? 'animate-spin' : ''}`} /> รีเฟรช
        </button>
      </div>

      {rows.length === 0 ? (
        <EmptyBlock label="ยังไม่มี send_log — pipeline ยังไม่ได้ส่ง (หรือ dry-run เท่านั้น)" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">เวลา</th>
                <th className="px-4 py-2.5 text-left font-medium">HCODE/Period</th>
                <th className="px-4 py-2.5 text-left font-medium">INSCL</th>
                <th className="px-4 py-2.5 text-left font-medium">Format</th>
                <th className="px-4 py-2.5 text-left font-medium">Endpoint</th>
                <th className="px-4 py-2.5 text-right font-medium">Duration</th>
                <th className="px-4 py-2.5 text-left font-medium">ผลลัพธ์</th>
                <th className="px-4 py-2.5 text-left font-medium">TxnID / Error</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(r => (
                <tr key={r.id} className="hover:bg-gray-50">
                  <td className="px-4 py-2 text-xs text-gray-500">{formatDT(r.sent_at)}</td>
                  <td className="px-4 py-2 font-mono text-xs">{r.hcode} / {r.period}</td>
                  <td className="px-4 py-2"><InsclBadge inscl={r.inscl} /></td>
                  <td className="px-4 py-2"><FormatBadge format={r.format as any} /></td>
                  <td className="px-4 py-2 font-mono text-[11px] text-gray-700">{r.endpoint}</td>
                  <td className="px-4 py-2 text-right font-mono text-[11px] text-gray-500">
                    {r.duration_ms != null ? `${r.duration_ms}ms` : '—'}
                  </td>
                  <td className="px-4 py-2"><SuccessPill success={r.success} /></td>
                  <td className="px-4 py-2 text-xs max-w-[280px]">
                    {r.success ? (
                      <code className="font-mono text-[11px] text-gray-700 truncate block" title={r.fdh_txn_id}>
                        {r.fdh_txn_id || '—'}
                      </code>
                    ) : (
                      <span className="text-red-700 text-[11px] block truncate" title={r.error_msg}>
                        {r.error_msg || '—'}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function SuccessPill({ success }: { success?: boolean }) {
  if (success === true) {
    return (
      <span className="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full bg-green-50 text-green-700">
        <CheckCircle2 className="w-3 h-3" /> สำเร็จ
      </span>
    )
  }
  if (success === false) {
    return (
      <span className="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full bg-red-50 text-red-700">
        <AlertCircle className="w-3 h-3" /> ล้มเหลว
      </span>
    )
  }
  return <span className="text-[11px] text-gray-400">—</span>
}

function formatDT(iso: string): string {
  try {
    return new Date(iso).toLocaleString('th-TH', { hour12: false, dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return iso
  }
}
