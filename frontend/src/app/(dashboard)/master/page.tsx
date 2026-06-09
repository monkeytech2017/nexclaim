'use client'
import { useState, useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { masterDataApi, type Icd10Master, type Icd9cmMaster, type TmtMaster } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { Search } from 'lucide-react'

type Tab = 'icd10' | 'icd9cm' | 'tmt'

const TAB_LABELS: Record<Tab, string> = {
  icd10:  'ICD-10',
  icd9cm: 'ICD-9CM',
  tmt:    'TMT',
}

const TAB_HINTS: Record<Tab, string> = {
  icd10:  '~40,000 รหัส',
  icd9cm: '~4,000 รหัส',
  tmt:    '~34,000 รายการ',
}

export default function MasterDataPage() {
  const [tab, setTab] = useState<Tab>('icd10')
  const [input, setInput] = useState('')
  const [q, setQ] = useState('')
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Debounce input → q by 300 ms
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => setQ(input.trim()), 300)
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [input])

  // Reset search when switching tabs
  const handleTabChange = (next: Tab) => {
    setTab(next)
    setInput('')
    setQ('')
  }

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        ตารางข้อมูลมาตรฐาน ICD-10 / ICD-9CM / TMT — ใช้ค้นหารหัสก่อน mapping ใน HIS
      </p>

      {/* Tabs */}
      <div className="flex gap-1 bg-gray-100 rounded-xl p-1 w-fit">
        {(Object.keys(TAB_LABELS) as Tab[]).map(t => (
          <button
            key={t}
            onClick={() => handleTabChange(t)}
            className={`text-xs px-4 py-1.5 rounded-lg font-medium transition-colors ${
              tab === t
                ? 'bg-white text-primary-600 shadow-sm'
                : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            {TAB_LABELS[t]}
            <span className="ml-1.5 text-[10px] font-normal text-gray-400">{TAB_HINTS[t]}</span>
          </button>
        ))}
      </div>

      {/* Search */}
      <div className="relative w-full max-w-md">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400 pointer-events-none" />
        <input
          type="text"
          value={input}
          onChange={e => setInput(e.target.value)}
          placeholder="ค้นหารหัสหรือชื่อ..."
          className="w-full pl-8 pr-3 py-2 text-xs border border-gray-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-primary-600 bg-white"
        />
      </div>

      {/* Results */}
      {tab === 'icd10'  && <Icd10Table  q={q} />}
      {tab === 'icd9cm' && <Icd9cmTable q={q} />}
      {tab === 'tmt'    && <TmtTable    q={q} />}
    </div>
  )
}

// ── ICD-10 table ──────────────────────────────────────────────────────────────

function Icd10Table({ q }: { q: string }) {
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['master', 'icd10', q],
    queryFn: () => masterDataApi.icd10({ q: q || undefined, limit: 50 }),
  })

  if (isLoading) return <LoadingBlock />
  if (isError)   return <ErrorBlock error={error} onRetry={() => refetch()} />

  const items = data?.items ?? []
  const count = data?.count ?? 0

  if (items.length === 0) return <EmptyBlock label="ไม่พบรายการ — ลองค้นหาด้วยรหัสหรือคำในชื่อ" />

  return (
    <ResultShell count={count} shown={items.length}>
      <table className="w-full text-xs">
        <thead>
          <tr className="bg-gray-50 text-gray-500">
            <th className="px-4 py-2.5 text-left font-medium w-24">รหัส</th>
            <th className="px-4 py-2.5 text-left font-medium">ชื่อไทย</th>
            <th className="px-4 py-2.5 text-left font-medium">ชื่ออังกฤษ</th>
            <th className="px-4 py-2.5 text-left font-medium w-24">Chapter</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-50">
          {items.map((row: Icd10Master) => (
            <tr key={row.code} className="hover:bg-gray-50">
              <td className="px-4 py-2 font-mono text-gray-700">{row.code}</td>
              <td className="px-4 py-2 text-gray-900">{row.name_th || '—'}</td>
              <td className="px-4 py-2 text-gray-600">{row.name_en || '—'}</td>
              <td className="px-4 py-2 text-gray-500">{row.chapter || '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </ResultShell>
  )
}

// ── ICD-9CM table ─────────────────────────────────────────────────────────────

function Icd9cmTable({ q }: { q: string }) {
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['master', 'icd9cm', q],
    queryFn: () => masterDataApi.icd9cm({ q: q || undefined, limit: 50 }),
  })

  if (isLoading) return <LoadingBlock />
  if (isError)   return <ErrorBlock error={error} onRetry={() => refetch()} />

  const items = data?.items ?? []
  const count = data?.count ?? 0

  if (items.length === 0) return <EmptyBlock label="ไม่พบรายการ — ลองค้นหาด้วยรหัสหรือคำในชื่อ" />

  return (
    <ResultShell count={count} shown={items.length}>
      <table className="w-full text-xs">
        <thead>
          <tr className="bg-gray-50 text-gray-500">
            <th className="px-4 py-2.5 text-left font-medium w-28">รหัส</th>
            <th className="px-4 py-2.5 text-left font-medium">ชื่อไทย</th>
            <th className="px-4 py-2.5 text-left font-medium">ชื่ออังกฤษ</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-50">
          {items.map((row: Icd9cmMaster) => (
            <tr key={row.code} className="hover:bg-gray-50">
              <td className="px-4 py-2 font-mono text-gray-700">{row.code}</td>
              <td className="px-4 py-2 text-gray-900">{row.name_th || '—'}</td>
              <td className="px-4 py-2 text-gray-600">{row.name_en || '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </ResultShell>
  )
}

// ── TMT table ─────────────────────────────────────────────────────────────────

function TmtTable({ q }: { q: string }) {
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['master', 'tmt', q],
    queryFn: () => masterDataApi.tmt({ q: q || undefined, limit: 50 }),
  })

  if (isLoading) return <LoadingBlock />
  if (isError)   return <ErrorBlock error={error} onRetry={() => refetch()} />

  const items = data?.items ?? []
  const count = data?.count ?? 0

  if (items.length === 0) return <EmptyBlock label="ไม่พบรายการ — ลองค้นหาด้วยรหัส TMT หรือชื่อยา" />

  return (
    <ResultShell count={count} shown={items.length}>
      <table className="w-full text-xs">
        <thead>
          <tr className="bg-gray-50 text-gray-500">
            <th className="px-4 py-2.5 text-left font-medium w-36">TMT Code</th>
            <th className="px-4 py-2.5 text-left font-medium">ชื่อ</th>
            <th className="px-4 py-2.5 text-left font-medium">Generic</th>
            <th className="px-4 py-2.5 text-left font-medium w-24">Strength</th>
            <th className="px-4 py-2.5 text-left font-medium w-28">Form</th>
            <th className="px-4 py-2.5 text-left font-medium w-20">Unit</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-50">
          {items.map((row: TmtMaster) => (
            <tr key={row.tmt_code} className="hover:bg-gray-50">
              <td className="px-4 py-2 font-mono text-gray-700">{row.tmt_code}</td>
              <td className="px-4 py-2 text-gray-900">{row.name_th || '—'}</td>
              <td className="px-4 py-2 text-gray-600">{row.generic_name || '—'}</td>
              <td className="px-4 py-2 text-gray-500">{row.strength || '—'}</td>
              <td className="px-4 py-2 text-gray-500">{row.dosage_form || '—'}</td>
              <td className="px-4 py-2 text-gray-500">{row.unit || '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </ResultShell>
  )
}

// ── Shared shell with count hint ──────────────────────────────────────────────

function ResultShell({ count, shown, children }: { count: number; shown: number; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <p className="text-[11px] text-gray-400">
        แสดง {shown} รายการแรก (จากทั้งหมด {count.toLocaleString()} รายการ) — พิมพ์เพื่อค้นหา/กรอง
      </p>
      <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
        {children}
      </div>
    </div>
  )
}
