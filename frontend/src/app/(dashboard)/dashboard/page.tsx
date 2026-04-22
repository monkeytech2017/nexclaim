'use client'
import { useQuery } from '@tanstack/react-query'
import Link from 'next/link'
import { opdApi, ipdApi } from '@/lib/api'
import { LoadingBlock, ErrorBlock } from '@/components/ui/feedback'
import { BatchStateBadge } from '@/components/ui/badges'
import { Inbox, FolderDown, History as HistoryIcon, CheckCircle } from 'lucide-react'

export default function DashboardPage() {
  const batches = useQuery({
    queryKey: ['opd-batches'],
    queryFn: opdApi.listBatches,
  })
  const imports = useQuery({
    queryKey: ['ipd-imports'],
    queryFn: ipdApi.listImports,
  })

  if (batches.isLoading || imports.isLoading) return <LoadingBlock />
  if (batches.isError) return <ErrorBlock error={batches.error} onRetry={() => batches.refetch()} />

  const batchList = batches.data?.batches ?? []
  const importList = imports.data?.imports ?? []
  const readyImports = importList.filter(i => i.ready)

  const pending = batchList.filter(b => b.state === 'RECEIVED').length
  const completed = batchList.filter(b => b.state === 'COMPLETED').length
  const failed = batchList.filter(b => b.state === 'FAILED').length

  return (
    <div className="flex flex-col gap-6">
      <div className="grid grid-cols-4 gap-4">
        <StatCard label="OPD Batches รอประมวล" value={pending} sub={`${batchList.length} batch ทั้งหมด`} icon={Inbox} color="text-blue-600" />
        <StatCard label="OPD สำเร็จ" value={completed} sub="COMPLETED" icon={CheckCircle} color="text-green-600" />
        <StatCard label="OPD ล้มเหลว" value={failed} sub="FAILED" icon={HistoryIcon} color="text-red-600" />
        <StatCard label="IPD Imports พร้อม" value={readyImports.length} sub={`${importList.length} folder ใน incoming/`} icon={FolderDown} color="text-purple-600" />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <section className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
            <span className="text-sm font-medium text-gray-900">OPD Batches ล่าสุด</span>
            <Link href="/opd-batches" className="text-xs text-primary-600 hover:text-primary-800">ดูทั้งหมด →</Link>
          </div>
          {batchList.length === 0 ? (
            <p className="p-6 text-center text-sm text-gray-400">ยังไม่มี batch — HIS จะ push ผ่าน POST /api/v1/his/opd/visits</p>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-gray-50 text-xs text-gray-500">
                  <th className="px-5 py-2.5 text-left font-medium">Batch ID</th>
                  <th className="px-5 py-2.5 text-left font-medium">Period</th>
                  <th className="px-5 py-2.5 text-right font-medium">Visits</th>
                  <th className="px-5 py-2.5 text-left font-medium">สถานะ</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {batchList.slice(0, 5).map(b => (
                  <tr key={b.batch_id} className="hover:bg-gray-50">
                    <td className="px-5 py-2.5 text-gray-700 font-mono text-xs">{b.batch_id}</td>
                    <td className="px-5 py-2.5 text-gray-600">{b.period}</td>
                    <td className="px-5 py-2.5 text-right text-gray-700">{b.visits?.length ?? 0}</td>
                    <td className="px-5 py-2.5"><BatchStateBadge state={b.state} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        <section className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
            <span className="text-sm font-medium text-gray-900">IPD Imports (folder)</span>
            <Link href="/ipd-imports" className="text-xs text-primary-600 hover:text-primary-800">จัดการ →</Link>
          </div>
          {importList.length === 0 ? (
            <p className="p-6 text-center text-sm text-gray-400">
              ไม่มี folder ใน incoming/ — หรือ IPD_SHARE_ROOT ยังไม่ได้ตั้ง
            </p>
          ) : (
            <ul className="divide-y divide-gray-50">
              {importList.slice(0, 8).map(i => (
                <li key={i.export_id} className="px-5 py-2.5 flex items-center justify-between text-sm">
                  <span className="font-mono text-xs text-gray-700">{i.export_id}</span>
                  <span className={`text-[11px] px-2 py-0.5 rounded-full ${i.ready ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
                    {i.ready ? 'พร้อมนำเข้า' : 'ยังไม่มี MANIFEST'}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </div>
  )
}

function StatCard({
  label, value, sub, icon: Icon, color,
}: {
  label: string; value: number; sub: string; icon: React.ComponentType<{ className?: string }>; color: string
}) {
  return (
    <div className="bg-white rounded-xl border border-gray-100 p-4">
      <div className="flex items-start justify-between">
        <div>
          <div className="text-xs text-gray-500 mb-1">{label}</div>
          <div className={`text-2xl font-semibold ${color}`}>{value}</div>
          <div className="text-xs text-gray-400 mt-1">{sub}</div>
        </div>
        <Icon className={`w-5 h-5 ${color} opacity-60`} />
      </div>
    </div>
  )
}
