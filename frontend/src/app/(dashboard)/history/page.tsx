'use client'
import { useQuery } from '@tanstack/react-query'
import { opdApi } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { BatchStateBadge, InsclBadge } from '@/components/ui/badges'

export default function HistoryPage() {
  const batches = useQuery({
    queryKey: ['opd-batches'],
    queryFn: opdApi.listBatches,
    refetchInterval: 15_000,
  })

  if (batches.isLoading) return <LoadingBlock />
  if (batches.isError) return <ErrorBlock error={batches.error} onRetry={() => batches.refetch()} />

  const list = (batches.data?.batches ?? [])
    .slice()
    .sort((a, b) => b.created_at.localeCompare(a.created_at))

  if (list.length === 0) {
    return <EmptyBlock label="ยังไม่มีประวัติ batch — เริ่มส่ง OPD visits จาก HIS หรือ import IPD จาก share folder" />
  }

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        รวม batch ทั้งหมด {list.length} รายการ (เรียงใหม่สุด → เก่าสุด) —
        <span className="ml-1">IPD imports ยังไม่ persist ใน history; ดูจาก <code className="bg-gray-100 px-1 rounded">processed/</code> บน disk</span>
      </p>
      <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-gray-50 text-xs text-gray-500">
              <th className="px-4 py-2.5 text-left font-medium">เวลา</th>
              <th className="px-4 py-2.5 text-left font-medium">Batch ID</th>
              <th className="px-4 py-2.5 text-left font-medium">HCODE</th>
              <th className="px-4 py-2.5 text-left font-medium">Period</th>
              <th className="px-4 py-2.5 text-left font-medium">INSCL</th>
              <th className="px-4 py-2.5 text-right font-medium">Visits</th>
              <th className="px-4 py-2.5 text-left font-medium">สถานะ</th>
              <th className="px-4 py-2.5 text-left font-medium">Note</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-50">
            {list.map(b => {
              const inscls = Array.from(new Set(b.visits?.map(v => v.inscl) ?? []))
              return (
                <tr key={b.batch_id} className="hover:bg-gray-50">
                  <td className="px-4 py-2.5 text-xs text-gray-500">{formatDT(b.created_at)}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-700">{b.batch_id}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-600">{b.hospital_code}</td>
                  <td className="px-4 py-2.5 text-gray-600">{b.period}</td>
                  <td className="px-4 py-2.5 flex gap-1">{inscls.map(i => <InsclBadge key={i} inscl={i} />)}</td>
                  <td className="px-4 py-2.5 text-right text-gray-700">{b.visits?.length ?? 0}</td>
                  <td className="px-4 py-2.5"><BatchStateBadge state={b.state} /></td>
                  <td className="px-4 py-2.5 text-xs text-gray-500 max-w-xs truncate" title={b.last_error}>
                    {b.last_error || b.exported_by || '—'}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
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
