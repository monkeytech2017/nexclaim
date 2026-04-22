'use client'
import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { submitDirect, INSCL_LABELS, type INSCL, type SubmitResponse } from '@/lib/api'
import { OutcomePanel } from '@/components/SubmissionOutcome'
import { ErrorBlock } from '@/components/ui/feedback'
import { Play } from 'lucide-react'

const INSCL_CODES: INSCL[] = [
  '011', 'WEL', 'LGO', 'OFC',
  'UCS', 'NON', 'WP1', 'WP2',
  'SSS', 'SS4',
  'TPBS', 'WK', 'MON', 'PRS',
]

export default function ClaimsPage() {
  const [form, setForm] = useState({
    inscl:  'UCS' as INSCL,
    period: currentPeriod(),
    hcode:  '12345',
    agency: '',
    dryRun: true,
  })

  const submit = useMutation({
    mutationFn: () => submitDirect({
      inscl:  form.inscl,
      period: form.period,
      hcode:  form.hcode || undefined,
      agency: form.agency || undefined,
      dryRun: form.dryRun,
    }),
  })

  return (
    <div className="max-w-4xl space-y-4">
      <div className="bg-amber-50 border border-amber-100 rounded-lg p-3 text-xs text-amber-900">
        <b>หมายเหตุ:</b> หน้านี้เรียก <code className="bg-amber-100/60 px-1 rounded">POST /api/submit</code> โดยตรง
        เหมาะสำหรับ <b>dev/admin ทดสอบ pipeline</b> — กรณีใช้งานจริงให้ใช้ OPD Batches หรือ IPD Imports แทน
        เพราะต้องมี data source ที่ wired จาก HIS หรือ share folder
      </div>

      <form
        onSubmit={e => { e.preventDefault(); submit.mutate() }}
        className="bg-white rounded-xl border border-gray-100 p-5 space-y-4"
      >
        <div className="grid grid-cols-4 gap-4">
          <Field label="INSCL">
            <select
              value={form.inscl}
              onChange={e => setForm(f => ({ ...f, inscl: e.target.value as INSCL }))}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm"
            >
              {INSCL_CODES.map(c => (
                <option key={c} value={c}>{c} — {INSCL_LABELS[c]}</option>
              ))}
            </select>
          </Field>
          <Field label="Period (YYYYMM)">
            <input
              type="text"
              value={form.period}
              onChange={e => setForm(f => ({ ...f, period: e.target.value }))}
              placeholder="202504"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono"
            />
          </Field>
          <Field label="HCODE">
            <input
              type="text"
              value={form.hcode}
              onChange={e => setForm(f => ({ ...f, hcode: e.target.value }))}
              placeholder="12345"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono"
            />
          </Field>
          <Field label="Agency (เฉพาะ OFC)">
            <input
              type="text"
              value={form.agency}
              onChange={e => setForm(f => ({ ...f, agency: e.target.value }))}
              placeholder="NBTC / BAAC / ..."
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm"
            />
          </Field>
        </div>

        <div className="flex items-center gap-4">
          <label className="flex items-center gap-2 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={form.dryRun}
              onChange={e => setForm(f => ({ ...f, dryRun: e.target.checked }))}
              className="rounded"
            />
            Dry-run (สร้าง zip แต่ไม่ส่ง FDH/CHI)
          </label>
          <button
            type="submit"
            disabled={submit.isPending}
            className="ml-auto text-sm px-4 py-2 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50 inline-flex items-center gap-1.5"
          >
            <Play className="w-3.5 h-3.5" />
            {submit.isPending ? 'กำลังส่ง...' : (form.dryRun ? 'Run dry-run' : 'Submit live')}
          </button>
        </div>
      </form>

      {submit.isError && <ErrorBlock error={submit.error} />}

      {submit.isSuccess && (
        <Result outcome={submit.data} />
      )}
    </div>
  )
}

function Result({ outcome }: { outcome: SubmitResponse }) {
  return (
    <div className="bg-white rounded-xl border border-gray-100 p-5 space-y-3">
      <h2 className="text-sm font-medium text-gray-900">ผลการ submit</h2>
      <OutcomePanel outcome={outcome} />
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-xs text-gray-500 mb-1">{label}</label>
      {children}
    </div>
  )
}

function currentPeriod(): string {
  const d = new Date()
  return `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}`
}
