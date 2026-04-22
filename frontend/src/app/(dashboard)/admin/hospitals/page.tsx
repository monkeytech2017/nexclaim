'use client'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { hospitalsApi, type Hospital } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { Pencil, Trash2, Plus, X } from 'lucide-react'

export default function HospitalsPage() {
  const qc = useQueryClient()
  const list = useQuery({
    queryKey: ['hospitals'],
    queryFn: hospitalsApi.list,
  })

  const [editing, setEditing] = useState<Hospital | null>(null)
  const [creating, setCreating] = useState(false)

  const save = useMutation({
    mutationFn: async (h: Hospital) => {
      if (creating) return hospitalsApi.create(h)
      return hospitalsApi.update(h.hcode, h)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['hospitals'] })
      setEditing(null)
      setCreating(false)
    },
  })

  const remove = useMutation({
    mutationFn: (hcode: string) => hospitalsApi.delete(hcode),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['hospitals'] }),
  })

  if (list.isLoading) return <LoadingBlock />
  if (list.isError) return <ErrorBlock error={list.error} onRetry={() => list.refetch()} />

  const rows = list.data?.hospitals ?? []

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs text-gray-500">
          จัดการรายการโรงพยาบาล (m_hospital) — ใช้เป็น FK สำหรับ m_doctor + his_* mapping
        </p>
        <button
          onClick={() => { setCreating(true); setEditing({ hcode: '', name_th: '', is_active: true }) }}
          className="text-xs px-3 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 inline-flex items-center gap-1.5"
        >
          <Plus className="w-3.5 h-3.5" /> เพิ่มโรงพยาบาล
        </button>
      </div>

      {rows.length === 0 ? (
        <EmptyBlock label="ยังไม่มีรายการ — กด 'เพิ่มโรงพยาบาล' เพื่อเริ่ม" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">HCODE</th>
                <th className="px-4 py-2.5 text-left font-medium">ชื่อ รพ.</th>
                <th className="px-4 py-2.5 text-left font-medium">จังหวัด/อำเภอ</th>
                <th className="px-4 py-2.5 text-left font-medium">HIS key</th>
                <th className="px-4 py-2.5 text-left font-medium">สถานะ</th>
                <th className="px-4 py-2.5 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(h => (
                <tr key={h.hcode} className="hover:bg-gray-50">
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-700">{h.hcode}</td>
                  <td className="px-4 py-2.5 text-gray-900">{h.name_th}</td>
                  <td className="px-4 py-2.5 text-gray-600 text-xs">
                    {[h.changwat, h.amphur].filter(Boolean).join(' / ') || '—'}
                  </td>
                  <td className="px-4 py-2.5 text-gray-500 text-xs font-mono">{h.his_db_key || '—'}</td>
                  <td className="px-4 py-2.5">
                    <span className={`text-[11px] px-2 py-0.5 rounded-full font-medium ${
                      h.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'
                    }`}>
                      {h.is_active ? 'ใช้งาน' : 'ปิด'}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-right space-x-1">
                    <button
                      onClick={() => { setCreating(false); setEditing(h) }}
                      className="text-xs p-1.5 rounded hover:bg-gray-100 text-gray-600"
                      title="แก้ไข"
                    >
                      <Pencil className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={() => {
                        if (confirm(`ลบโรงพยาบาล ${h.hcode} (${h.name_th})?`)) {
                          remove.mutate(h.hcode)
                        }
                      }}
                      className="text-xs p-1.5 rounded hover:bg-red-50 text-red-600"
                      title="ลบ"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {save.isError && <ErrorBlock error={save.error} />}
      {remove.isError && <ErrorBlock error={remove.error} />}

      {editing && (
        <HospitalDialog
          initial={editing}
          isCreate={creating}
          busy={save.isPending}
          onClose={() => { setEditing(null); setCreating(false) }}
          onSubmit={h => save.mutate(h)}
        />
      )}
    </div>
  )
}

function HospitalDialog({
  initial, isCreate, busy, onClose, onSubmit,
}: {
  initial: Hospital
  isCreate: boolean
  busy: boolean
  onClose: () => void
  onSubmit: (h: Hospital) => void
}) {
  const [form, setForm] = useState<Hospital>(initial)
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <form
        onSubmit={e => { e.preventDefault(); onSubmit(form) }}
        className="bg-white rounded-xl w-[520px] max-w-[92vw] shadow-xl"
      >
        <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
          <h2 className="text-sm font-medium text-gray-900">
            {isCreate ? 'เพิ่มโรงพยาบาล' : `แก้ไข ${initial.hcode}`}
          </h2>
          <button type="button" onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="px-5 py-4 grid grid-cols-2 gap-3">
          <Field label="HCODE (5 หลัก)">
            <input
              type="text" required maxLength={5} minLength={5}
              disabled={!isCreate}
              value={form.hcode}
              onChange={e => setForm({ ...form, hcode: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50 disabled:text-gray-500"
            />
          </Field>
          <Field label="ชื่อ รพ. (TH)">
            <input
              type="text" required
              value={form.name_th}
              onChange={e => setForm({ ...form, name_th: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm"
            />
          </Field>
          <Field label="จังหวัด (2 หลัก)">
            <input
              type="text" maxLength={2}
              value={form.changwat ?? ''}
              onChange={e => setForm({ ...form, changwat: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono"
            />
          </Field>
          <Field label="อำเภอ (4 หลัก)">
            <input
              type="text" maxLength={4}
              value={form.amphur ?? ''}
              onChange={e => setForm({ ...form, amphur: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono"
            />
          </Field>
          <Field label="HIS DB key" span={2}>
            <input
              type="text"
              value={form.his_db_key ?? ''}
              placeholder="เช่น hospital_a (ใช้เลือก HIS connection per รพ.)"
              onChange={e => setForm({ ...form, his_db_key: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm"
            />
          </Field>
          <label className="col-span-2 flex items-center gap-2 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={form.is_active}
              onChange={e => setForm({ ...form, is_active: e.target.checked })}
              className="rounded"
            />
            ใช้งานอยู่
          </label>
        </div>
        <div className="px-5 py-3 border-t border-gray-100 flex items-center justify-end gap-2">
          <button type="button" onClick={onClose}
            className="text-sm px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700">
            ยกเลิก
          </button>
          <button type="submit" disabled={busy}
            className="text-sm px-4 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50">
            {busy ? 'กำลังบันทึก...' : 'บันทึก'}
          </button>
        </div>
      </form>
    </div>
  )
}

function Field({ label, span = 1, children }: { label: string; span?: 1 | 2; children: React.ReactNode }) {
  return (
    <div className={span === 2 ? 'col-span-2' : undefined}>
      <label className="block text-xs text-gray-500 mb-1">{label}</label>
      {children}
    </div>
  )
}
