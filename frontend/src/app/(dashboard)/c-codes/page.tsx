'use client'
import { useState, Suspense } from 'react'
import { useSearchParams, useRouter } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ccodesApi, hospitalsApi } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { RefreshCw, CheckCircle2, Download, X } from 'lucide-react'

export default function CCodesPage() {
  return (
    <Suspense fallback={<LoadingBlock />}>
      <CCodesContent />
    </Suspense>
  )
}

function CCodesContent() {
  const qc = useQueryClient()
  const searchParams = useSearchParams()
  const router = useRouter()
  const batchFromURL = searchParams.get('batch') ?? ''

  const [filterHcode, setFilterHcode] = useState('')
  const [filterPeriod, setFilterPeriod] = useState('')
  const [filterResolved, setFilterResolved] = useState<'' | 'true' | 'false'>('false')
  const [filterCode, setFilterCode] = useState('')

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })
  const list = useQuery({
    queryKey: ['ccodes', batchFromURL, filterHcode, filterPeriod, filterResolved, filterCode],
    queryFn: () => ccodesApi.list({
      batch_id: batchFromURL || undefined,
      hcode:    filterHcode || undefined,
      period:   filterPeriod || undefined,
      resolved: filterResolved === '' ? undefined : filterResolved === 'true',
      c_code:   filterCode || undefined,
    }),
  })

  const fetchRep = useMutation({
    mutationFn: ({ hcode, period }: { hcode: string; period: string }) =>
      ccodesApi.fetchRep(hcode, period),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['ccodes'] }),
  })
  const resolve = useMutation({
    mutationFn: ({ id, by }: { id: string; by: string }) => ccodesApi.resolve(id, by),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['ccodes'] }),
  })

  if (list.isLoading) return <LoadingBlock />
  if (list.isError) return <ErrorBlock error={list.error} onRetry={() => list.refetch()} />

  const rows = list.data?.items ?? []
  const hcodes = hospitals.data?.hospitals.map(h => h.hcode) ?? []

  const canFetch = filterHcode && filterPeriod

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        C-code คือข้อผิดพลาด per-record ที่ FDH/CHI ตอบกลับใน REP.xml.
        หลังส่ง claim ใน period → รอ FDH process เสร็จ → กด "Fetch REP" เพื่อดึงผลลัพธ์.
      </p>

      {batchFromURL && (
        <div className="bg-blue-50 border border-blue-100 rounded-lg px-3 py-2 text-xs text-blue-900 flex items-center gap-2">
          <span>filtering to batch:</span>
          <code className="font-mono bg-white px-1.5 py-0.5 rounded border border-blue-200">{batchFromURL}</code>
          <button onClick={() => router.push('/c-codes')}
            className="ml-auto inline-flex items-center gap-1 text-blue-700 hover:text-blue-900">
            <X className="w-3 h-3" /> clear
          </button>
        </div>
      )}

      <div className="flex items-center gap-2 flex-wrap">
        <select value={filterHcode}
          onChange={e => setFilterHcode(e.target.value)}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1">
          <option value="">เลือก รพ.</option>
          {hcodes.map(h => <option key={h} value={h}>{h}</option>)}
        </select>
        <input
          type="text" value={filterPeriod}
          onChange={e => setFilterPeriod(e.target.value)}
          placeholder="YYYYMM"
          maxLength={6}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1 font-mono w-24"
        />
        <select value={filterResolved}
          onChange={e => setFilterResolved(e.target.value as '' | 'true' | 'false')}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1">
          <option value="">ทั้งหมด</option>
          <option value="false">ยังไม่แก้</option>
          <option value="true">แก้แล้ว</option>
        </select>
        <input
          type="text" value={filterCode}
          onChange={e => setFilterCode(e.target.value)}
          placeholder="C-code (เช่น C104)"
          className="text-xs border border-gray-200 rounded-lg px-2 py-1 font-mono w-32"
        />
        <span className="text-xs text-gray-400">{rows.length} รายการ</span>

        <div className="ml-auto flex items-center gap-2">
          <button
            onClick={() => list.refetch()}
            className="text-xs px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-600 inline-flex items-center gap-1.5"
            disabled={list.isFetching}
          >
            <RefreshCw className={`w-3.5 h-3.5 ${list.isFetching ? 'animate-spin' : ''}`} /> รีเฟรช
          </button>
          <button
            disabled={!canFetch || fetchRep.isPending}
            onClick={() => fetchRep.mutate({ hcode: filterHcode, period: filterPeriod })}
            className="text-xs px-3 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50 inline-flex items-center gap-1.5"
          >
            <Download className="w-3.5 h-3.5" />
            {fetchRep.isPending ? 'กำลังดึง REP...' : `Fetch REP (${filterHcode || '—'} / ${filterPeriod || 'YYYYMM'})`}
          </button>
        </div>
      </div>

      {fetchRep.isSuccess && (
        <div className="bg-green-50 border border-green-100 rounded p-2 text-xs text-green-900">
          ดึง REP สำเร็จ — fetched {fetchRep.data.fetched},
          inserted {fetchRep.data.inserted}
          {fetchRep.data.skipped > 0 && <>, skipped {fetchRep.data.skipped} (no matching batch)</>}
          {fetchRep.data.errors > 0 && <>, errors {fetchRep.data.errors}</>}
        </div>
      )}
      {fetchRep.isError && <ErrorBlock error={fetchRep.error} />}

      {rows.length === 0 ? (
        <EmptyBlock label={canFetch
          ? 'ยังไม่มี c-code — ลอง Fetch REP เพื่อดึงจาก FDH'
          : 'เลือก รพ. + period แล้วกด Fetch REP เพื่อเริ่ม'} />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">เวลา</th>
                <th className="px-4 py-2.5 text-left font-medium">C-code</th>
                <th className="px-4 py-2.5 text-left font-medium">HN</th>
                <th className="px-4 py-2.5 text-left font-medium">AN/SEQ</th>
                <th className="px-4 py-2.5 text-left font-medium">Field</th>
                <th className="px-4 py-2.5 text-left font-medium">Value</th>
                <th className="px-4 py-2.5 text-left font-medium">รายละเอียด</th>
                <th className="px-4 py-2.5 text-left font-medium">สถานะ</th>
                <th className="px-4 py-2.5 text-right font-medium">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(c => (
                <tr key={c.id} className="hover:bg-gray-50">
                  <td className="px-4 py-2.5 text-xs text-gray-500">{formatDT(c.received_at)}</td>
                  <td className="px-4 py-2.5">
                    <code className="text-xs bg-red-50 text-red-700 px-2 py-0.5 rounded font-mono">{c.c_code}</code>
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs">{c.hn || '—'}</td>
                  <td className="px-4 py-2.5 font-mono text-xs">{c.an_or_seq || '—'}</td>
                  <td className="px-4 py-2.5 text-xs text-gray-700">{c.field_name || '—'}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-500">{c.field_value || '—'}</td>
                  <td className="px-4 py-2.5 text-xs text-gray-700 max-w-xs truncate" title={c.c_desc}>
                    {c.c_desc || '—'}
                  </td>
                  <td className="px-4 py-2.5">
                    {c.resolved ? (
                      <span className="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full bg-green-50 text-green-700">
                        <CheckCircle2 className="w-3 h-3" /> แก้แล้ว
                      </span>
                    ) : (
                      <span className="text-[11px] px-2 py-0.5 rounded-full bg-amber-50 text-amber-700">ยังไม่แก้</span>
                    )}
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    {!c.resolved && (
                      <button
                        disabled={resolve.isPending}
                        onClick={() => {
                          const by = prompt('ผู้แก้ไข (ชื่อ / email)') ?? ''
                          if (by) resolve.mutate({ id: c.id, by })
                        }}
                        className="text-xs px-2.5 py-1 rounded border border-gray-200 hover:bg-gray-50 text-gray-700 disabled:opacity-50"
                      >
                        Mark resolved
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {resolve.isError && <ErrorBlock error={resolve.error} />}
    </div>
  )
}

function formatDT(iso: string): string {
  try {
    return new Date(iso).toLocaleString('th-TH', { hour12: false, dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return iso
  }
}
