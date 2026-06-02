'use client'
import { useEffect, useState } from 'react'
import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { tmtApi } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { Search, ChevronLeft, ChevronRight } from 'lucide-react'

export default function TmtMasterPage() {
  const [input, setInput] = useState('')
  const [q, setQ] = useState('')
  const [limit, setLimit] = useState(50)
  const [offset, setOffset] = useState(0)

  // debounce ~300ms: input → q (and reset paging on a new search)
  useEffect(() => {
    const t = setTimeout(() => {
      setQ(input.trim())
      setOffset(0)
    }, 300)
    return () => clearTimeout(t)
  }, [input])

  const query = useQuery({
    queryKey: ['tmt', q, limit, offset],
    queryFn: () => tmtApi.search({ q: q || undefined, limit, offset }),
    placeholderData: keepPreviousData,
  })

  const items = query.data?.items ?? []
  const total = query.data?.total ?? 0
  const showingFrom = total === 0 ? 0 : offset + 1
  const showingTo = offset + items.length

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        ทะเบียนยามาตรฐาน TMT 24 หลัก (~34,000 รายการ). ค้นด้วยรหัส / ชื่อ / generic — read-only.
      </p>

      <div className="flex items-center justify-between gap-3">
        <div className="relative flex-1 max-w-md">
          <Search className="w-3.5 h-3.5 text-gray-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            value={input}
            onChange={e => setInput(e.target.value)}
            placeholder="ค้นหา TMT code / ชื่อยา / generic..."
            className="w-full text-xs border border-gray-200 rounded-lg pl-8 pr-3 py-2"
          />
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500">ต่อหน้า:</span>
          <select
            value={limit}
            onChange={e => { setLimit(Number(e.target.value)); setOffset(0) }}
            className="text-xs border border-gray-200 rounded-lg px-2 py-1.5"
          >
            <option value={50}>50</option>
            <option value={100}>100</option>
            <option value={200}>200</option>
          </select>
        </div>
      </div>

      {query.isLoading ? (
        <LoadingBlock />
      ) : query.isError ? (
        <ErrorBlock error={query.error} onRetry={() => query.refetch()} />
      ) : items.length === 0 ? (
        <EmptyBlock label="ไม่พบยา — ลองค้นคำอื่น" />
      ) : (
        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs text-gray-500">
            <span>
              แสดง {showingFrom}–{showingTo} จาก {total.toLocaleString('th-TH')} รายการ
              {query.isFetching && <span className="ml-2 text-gray-400">(กำลังโหลด...)</span>}
            </span>
            <div className="flex items-center gap-1">
              <button
                onClick={() => setOffset(Math.max(0, offset - limit))}
                disabled={offset === 0}
                className="inline-flex items-center gap-1 px-2 py-1 rounded-lg border border-gray-200 hover:bg-gray-50 disabled:opacity-40"
              >
                <ChevronLeft className="w-3.5 h-3.5" /> ก่อนหน้า
              </button>
              <button
                onClick={() => setOffset(offset + limit)}
                disabled={offset + limit >= total}
                className="inline-flex items-center gap-1 px-2 py-1 rounded-lg border border-gray-200 hover:bg-gray-50 disabled:opacity-40"
              >
                ถัดไป <ChevronRight className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-gray-50 text-xs text-gray-500">
                  <th className="px-4 py-2.5 text-left font-medium">TMT Code</th>
                  <th className="px-4 py-2.5 text-left font-medium">ชื่อ</th>
                  <th className="px-4 py-2.5 text-left font-medium">Generic</th>
                  <th className="px-4 py-2.5 text-left font-medium">Strength</th>
                  <th className="px-4 py-2.5 text-left font-medium">Form</th>
                  <th className="px-4 py-2.5 text-left font-medium">Unit</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {items.map(d => (
                  <tr key={d.tmt_code} className="hover:bg-gray-50">
                    <td className="px-4 py-2.5 font-mono text-[11px] text-gray-600">{d.tmt_code}</td>
                    <td className="px-4 py-2.5 text-gray-900">{d.name_th || '—'}</td>
                    <td className="px-4 py-2.5 text-xs text-gray-600">{d.generic_name || '—'}</td>
                    <td className="px-4 py-2.5 text-xs text-gray-600">{d.strength || '—'}</td>
                    <td className="px-4 py-2.5 text-xs text-gray-600">{d.dosage_form || '—'}</td>
                    <td className="px-4 py-2.5 text-xs text-gray-600">{d.unit || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
