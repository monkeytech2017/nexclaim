'use client'
import { useMemo, useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import {
  BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, Tooltip, Legend, ResponsiveContainer, CartesianGrid,
} from 'recharts'
import {
  Inbox, CheckCircle2, AlertTriangle, Timer,
} from 'lucide-react'
import { dashboardApi, hospitalsApi } from '@/lib/api'
import { useIdentity } from '@/lib/auth-context'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'

// Colors aligned with FormatBadge palette (badges.tsx).
const FORMAT_COLORS: Record<string, string> = {
  '16FILES': '#9333ea', // purple
  CIPN:      '#2563eb', // blue
  CSOP:      '#0284c7', // sky
  AIPN:      '#4f46e5', // indigo
  SSOP:      '#7c3aed', // violet
}
const FORMAT_KEYS = ['16FILES', 'CIPN', 'CSOP', 'AIPN', 'SSOP'] as const

const STATUS_COLORS: Record<string, string> = {
  sent:    '#16a34a', // green
  pending: '#6b7280', // gray
  error:   '#dc2626', // red
}

export default function DashboardPage() {
  const router = useRouter()
  const identity = useIdentity()
  const isHospital = identity?.role === 'hospital'

  const [hcode, setHcode] = useState(isHospital ? identity.hcode ?? '' : '')
  const [periodFrom, setPeriodFrom] = useState('')
  const [periodTo, setPeriodTo] = useState('')

  const hospitals = useQuery({
    queryKey: ['hospitals'],
    queryFn:  hospitalsApi.list,
  })

  const stats = useQuery({
    queryKey: ['dashboard-stats', { hcode, periodFrom, periodTo }],
    queryFn:  () => dashboardApi.stats({
      hcode:       hcode       || undefined,
      period_from: periodFrom  || undefined,
      period_to:   periodTo    || undefined,
    }),
    refetchInterval: 30_000,
  })

  // Flatten by_day into recharts-friendly rows: {day, "16FILES": n, CIPN: n, ...}.
  const dailyRows = useMemo(() => {
    const rows = stats.data?.by_day ?? []
    return rows.map(entry => {
      const row: Record<string, string | number> = { day: entry.day }
      for (const k of FORMAT_KEYS) row[k] = entry.formats?.[k] ?? 0
      return row
    })
  }, [stats.data])

  const topCcodeRows = useMemo(() => {
    const rows = stats.data?.top_ccodes ?? []
    return rows.map(c => ({
      c_code:  c.c_code,
      count:   c.count,
      label:   `${c.c_code}: ${truncate(c.desc_sample, 30)}`,
    }))
  }, [stats.data])

  return (
    <div className="flex flex-col gap-6">
      {/* Filters */}
      <section className="bg-white rounded-xl border border-gray-100 p-4">
        <div className="grid grid-cols-4 gap-3 items-end">
          <div>
            <label className="block text-xs text-gray-500 mb-1">โรงพยาบาล</label>
            <select
              value={hcode}
              onChange={e => setHcode(e.target.value)}
              disabled={isHospital}
              className="w-full text-xs border border-gray-200 rounded-md px-2 py-1.5 bg-white disabled:bg-gray-50 disabled:text-gray-500"
            >
              <option value="">ทั้งหมด</option>
              {(hospitals.data?.hospitals ?? []).map(h => (
                <option key={h.hcode} value={h.hcode}>
                  {h.hcode} — {h.name_th}
                </option>
              ))}
              {isHospital && hcode && !(hospitals.data?.hospitals ?? []).some(h => h.hcode === hcode) && (
                <option value={hcode}>{hcode}</option>
              )}
            </select>
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Period จาก (YYYYMM)</label>
            <input
              value={periodFrom}
              onChange={e => setPeriodFrom(e.target.value)}
              placeholder="202504"
              className="w-full text-xs border border-gray-200 rounded-md px-2 py-1.5"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Period ถึง (YYYYMM)</label>
            <input
              value={periodTo}
              onChange={e => setPeriodTo(e.target.value)}
              placeholder="202504"
              className="w-full text-xs border border-gray-200 rounded-md px-2 py-1.5"
            />
          </div>
          <div className="text-xs text-gray-400 pb-1.5">
            {stats.isFetching ? 'กำลังรีเฟรช...' : 'รีเฟรชอัตโนมัติทุก 30 วินาที'}
          </div>
        </div>
      </section>

      {stats.isLoading ? (
        <LoadingBlock />
      ) : stats.isError ? (
        <ErrorBlock error={stats.error} onRetry={() => stats.refetch()} />
      ) : (stats.data?.summary.batches_total ?? 0) === 0 ? (
        <EmptyBlock label="ยังไม่มี submission — ส่ง claim สักครั้งก่อน" />
      ) : (
        <>
          {/* Top row — KPIs */}
          <div className="grid grid-cols-4 gap-4">
            <StatCard
              label="Batches ทั้งหมด"
              value={fmt(stats.data!.summary.batches_total)}
              sub={`${pct(stats.data!.summary.send_success_rate)} success rate`}
              icon={Inbox}
              color="text-blue-600"
            />
            <StatCard
              label="Records ส่งสำเร็จ"
              value={fmt(stats.data!.summary.records_total - stats.data!.summary.records_errors)}
              sub={`${fmt(stats.data!.summary.records_total)} records ทั้งหมด`}
              icon={CheckCircle2}
              color="text-green-600"
            />
            <StatCard
              label="C-code ค้าง"
              value={fmt(stats.data!.summary.ccodes_open)}
              sub="ยังไม่ resolved"
              icon={AlertTriangle}
              color="text-red-600"
            />
            <StatCard
              label="Avg send latency"
              value={`${fmt(stats.data!.summary.avg_send_ms)} ms`}
              sub="FDH + CHI เฉลี่ย"
              icon={Timer}
              color="text-purple-600"
            />
          </div>

          {/* Second row — submissions over time + status distribution */}
          <div className="grid grid-cols-2 gap-4">
            <section className="bg-white rounded-xl border border-gray-100 overflow-hidden">
              <div className="px-5 py-3.5 border-b border-gray-100">
                <span className="text-sm font-medium text-gray-900">Submissions over time (30 วัน)</span>
              </div>
              <div className="p-4" style={{ height: 300 }}>
                {dailyRows.length === 0 ? (
                  <div className="h-full flex items-center justify-center text-xs text-gray-400">
                    ไม่มีข้อมูล
                  </div>
                ) : (
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={dailyRows} margin={{ top: 8, right: 12, left: 0, bottom: 0 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="#f3f4f6" />
                      <XAxis dataKey="day" tick={{ fontSize: 10 }} tickFormatter={shortDay} />
                      <YAxis tick={{ fontSize: 10 }} allowDecimals={false} />
                      <Tooltip
                        contentStyle={{ fontSize: 11 }}
                        labelFormatter={d => new Date(String(d)).toLocaleDateString('th-TH', { year: 'numeric', month: 'short', day: 'numeric' })}
                      />
                      <Legend wrapperStyle={{ fontSize: 11 }} />
                      {FORMAT_KEYS.map(fk => (
                        <Bar key={fk} dataKey={fk} stackId="a" fill={FORMAT_COLORS[fk]} />
                      ))}
                    </BarChart>
                  </ResponsiveContainer>
                )}
              </div>
            </section>

            <section className="bg-white rounded-xl border border-gray-100 overflow-hidden">
              <div className="px-5 py-3.5 border-b border-gray-100">
                <span className="text-sm font-medium text-gray-900">Status distribution</span>
              </div>
              <div className="p-4" style={{ height: 300 }}>
                {(stats.data?.by_status?.length ?? 0) === 0 ? (
                  <div className="h-full flex items-center justify-center text-xs text-gray-400">
                    ไม่มีข้อมูล
                  </div>
                ) : (
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Tooltip contentStyle={{ fontSize: 11 }} />
                      <Legend wrapperStyle={{ fontSize: 11 }} />
                      <Pie
                        data={stats.data!.by_status}
                        dataKey="count"
                        nameKey="status"
                        innerRadius={60}
                        outerRadius={95}
                        paddingAngle={2}
                      >
                        {stats.data!.by_status.map(entry => (
                          <Cell
                            key={entry.status}
                            fill={STATUS_COLORS[entry.status] ?? '#9ca3af'}
                          />
                        ))}
                      </Pie>
                    </PieChart>
                  </ResponsiveContainer>
                )}
              </div>
            </section>
          </div>

          {/* Third row — top c-codes + by format table */}
          <div className="grid grid-cols-2 gap-4">
            <section className="bg-white rounded-xl border border-gray-100 overflow-hidden">
              <div className="px-5 py-3.5 border-b border-gray-100">
                <span className="text-sm font-medium text-gray-900">Top C-codes (คลิกเพื่อดูรายละเอียด)</span>
              </div>
              <div className="p-4" style={{ height: 320 }}>
                {topCcodeRows.length === 0 ? (
                  <div className="h-full flex items-center justify-center text-xs text-gray-400">
                    ไม่มี C-code
                  </div>
                ) : (
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart
                      data={topCcodeRows}
                      layout="vertical"
                      margin={{ top: 8, right: 16, left: 8, bottom: 0 }}
                    >
                      <CartesianGrid strokeDasharray="3 3" stroke="#f3f4f6" />
                      <XAxis type="number" tick={{ fontSize: 10 }} allowDecimals={false} />
                      <YAxis
                        type="category"
                        dataKey="label"
                        tick={{ fontSize: 10 }}
                        width={200}
                      />
                      <Tooltip contentStyle={{ fontSize: 11 }} />
                      <Bar
                        dataKey="count"
                        fill="#dc2626"
                        cursor="pointer"
                        onClick={(data: { c_code?: string }) => {
                          if (data?.c_code) router.push(`/c-codes?c_code=${encodeURIComponent(data.c_code)}`)
                        }}
                      />
                    </BarChart>
                  </ResponsiveContainer>
                )}
              </div>
            </section>

            <section className="bg-white rounded-xl border border-gray-100 overflow-hidden">
              <div className="px-5 py-3.5 border-b border-gray-100">
                <span className="text-sm font-medium text-gray-900">สรุปตาม Format</span>
              </div>
              {(stats.data?.by_format?.length ?? 0) === 0 ? (
                <p className="p-6 text-center text-sm text-gray-400">ไม่มีข้อมูล</p>
              ) : (
                <table className="w-full text-xs">
                  <thead>
                    <tr className="bg-gray-50 text-gray-500">
                      <th className="px-5 py-2.5 text-left font-medium">Format</th>
                      <th className="px-5 py-2.5 text-right font-medium">Batches</th>
                      <th className="px-5 py-2.5 text-right font-medium">Records</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-50">
                    {stats.data!.by_format.map(row => (
                      <tr key={row.format} className="hover:bg-gray-50">
                        <td className="px-5 py-2.5">
                          <Link
                            href={`/submissions?format=${encodeURIComponent(row.format)}`}
                            className="inline-flex items-center gap-2 font-mono font-medium text-primary-600 hover:text-primary-800"
                          >
                            <span
                              className="inline-block w-2 h-2 rounded-full"
                              style={{ backgroundColor: FORMAT_COLORS[row.format] ?? '#9ca3af' }}
                            />
                            {row.format}
                          </Link>
                        </td>
                        <td className="px-5 py-2.5 text-right text-gray-700">{fmt(row.count)}</td>
                        <td className="px-5 py-2.5 text-right text-gray-700">{fmt(row.records)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>
          </div>
        </>
      )}
    </div>
  )
}

function StatCard({
  label, value, sub, icon: Icon, color,
}: {
  label: string
  value: string
  sub:   string
  icon:  React.ComponentType<{ className?: string }>
  color: string
}) {
  return (
    <div className="bg-white rounded-xl border border-gray-100 p-4">
      <div className="flex items-start justify-between">
        <div className="min-w-0">
          <div className="text-xs text-gray-500 mb-1">{label}</div>
          <div className={`text-2xl font-semibold ${color} truncate`}>{value}</div>
          <div className="text-xs text-gray-400 mt-1">{sub}</div>
        </div>
        <Icon className={`w-5 h-5 ${color} opacity-60`} />
      </div>
    </div>
  )
}

// ── helpers ──

function fmt(n: number): string {
  return new Intl.NumberFormat('th-TH').format(n)
}

function pct(x: number): string {
  if (!Number.isFinite(x)) return '—'
  return `${(x * 100).toFixed(1)}%`
}

function truncate(s: string, max: number): string {
  if (!s) return ''
  return s.length > max ? s.slice(0, max - 1) + '…' : s
}

function shortDay(d: string): string {
  // "2026-04-23" → "23/4"
  const parts = d.split('-')
  if (parts.length !== 3) return d
  return `${parts[2]}/${Number(parts[1])}`
}
