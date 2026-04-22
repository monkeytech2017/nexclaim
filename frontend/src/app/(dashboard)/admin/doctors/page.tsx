'use client'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { doctorsApi, hospitalsApi, type Doctor } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { Pencil, Trash2, Plus, X } from 'lucide-react'

export default function DoctorsPage() {
  const qc = useQueryClient()
  const [filterHcode, setFilterHcode] = useState('')

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })
  const list = useQuery({
    queryKey: ['doctors', filterHcode],
    queryFn: () => doctorsApi.list(filterHcode || undefined),
  })

  const [editing, setEditing] = useState<Doctor | null>(null)
  const [creating, setCreating] = useState(false)

  const save = useMutation({
    mutationFn: async (d: Doctor) => (creating ? doctorsApi.create(d) : doctorsApi.update(d.doctor_id, d)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['doctors'] })
      setEditing(null); setCreating(false)
    },
  })
  const remove = useMutation({
    mutationFn: (id: string) => doctorsApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['doctors'] }),
  })

  if (list.isLoading) return <LoadingBlock />
  if (list.isError) return <ErrorBlock error={list.error} onRetry={() => list.refetch()} />

  const rows = list.data?.doctors ?? []
  const hcodes = hospitals.data?.hospitals.map(h => h.hcode) ?? []

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500">โรงพยาบาล:</span>
          <select
            value={filterHcode}
            onChange={e => setFilterHcode(e.target.value)}
            className="text-xs border border-gray-200 rounded-lg px-2 py-1"
          >
            <option value="">ทั้งหมด</option>
            {hcodes.map(h => <option key={h} value={h}>{h}</option>)}
          </select>
        </div>
        <button
          onClick={() => {
            setCreating(true)
            setEditing({ doctor_id: '', hcode: hcodes[0] ?? '', license_no: '', is_active: true })
          }}
          disabled={hcodes.length === 0}
          className="text-xs px-3 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50 inline-flex items-center gap-1.5"
        >
          <Plus className="w-3.5 h-3.5" /> เพิ่มแพทย์
        </button>
      </div>
      {hcodes.length === 0 && (
        <p className="text-xs text-amber-700 bg-amber-50 border border-amber-100 rounded px-3 py-2">
          ยังไม่มีรายการโรงพยาบาล — เพิ่มที่ <span className="underline">/admin/hospitals</span> ก่อน
        </p>
      )}

      {rows.length === 0 ? (
        <EmptyBlock label="ยังไม่มีแพทย์ในฐานข้อมูล" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">Doctor ID</th>
                <th className="px-4 py-2.5 text-left font-medium">HCODE</th>
                <th className="px-4 py-2.5 text-left font-medium">ใบประกอบฯ (DRDX)</th>
                <th className="px-4 py-2.5 text-left font-medium">ชื่อ</th>
                <th className="px-4 py-2.5 text-left font-medium">สาขา</th>
                <th className="px-4 py-2.5 text-left font-medium">สถานะ</th>
                <th className="px-4 py-2.5 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(d => (
                <tr key={d.doctor_id} className="hover:bg-gray-50">
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-700">{d.doctor_id}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-600">{d.hcode}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-700">{d.license_no}</td>
                  <td className="px-4 py-2.5 text-gray-900">{d.name_th || '—'}</td>
                  <td className="px-4 py-2.5 text-gray-600 text-xs">{d.specialty || '—'}</td>
                  <td className="px-4 py-2.5">
                    <span className={`text-[11px] px-2 py-0.5 rounded-full font-medium ${
                      d.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'
                    }`}>
                      {d.is_active ? 'ใช้งาน' : 'ปิด'}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-right space-x-1">
                    <button onClick={() => { setCreating(false); setEditing(d) }}
                      className="p-1.5 rounded hover:bg-gray-100 text-gray-600">
                      <Pencil className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={() => {
                        if (confirm(`ลบแพทย์ ${d.doctor_id} (${d.name_th || '—'})?`)) remove.mutate(d.doctor_id)
                      }}
                      className="p-1.5 rounded hover:bg-red-50 text-red-600">
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
        <DoctorDialog
          initial={editing} isCreate={creating}
          hcodes={hcodes} busy={save.isPending}
          onClose={() => { setEditing(null); setCreating(false) }}
          onSubmit={d => save.mutate(d)}
        />
      )}
    </div>
  )
}

function DoctorDialog({
  initial, isCreate, hcodes, busy, onClose, onSubmit,
}: {
  initial: Doctor; isCreate: boolean; hcodes: string[]; busy: boolean
  onClose: () => void; onSubmit: (d: Doctor) => void
}) {
  const [form, setForm] = useState<Doctor>(initial)
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <form onSubmit={e => { e.preventDefault(); onSubmit(form) }}
        className="bg-white rounded-xl w-[520px] max-w-[92vw] shadow-xl">
        <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
          <h2 className="text-sm font-medium text-gray-900">
            {isCreate ? 'เพิ่มแพทย์' : `แก้ไข ${initial.doctor_id}`}
          </h2>
          <button type="button" onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="px-5 py-4 grid grid-cols-2 gap-3">
          <Field label="Doctor ID (internal HIS)">
            <input type="text" required disabled={!isCreate}
              value={form.doctor_id}
              onChange={e => setForm({ ...form, doctor_id: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50 disabled:text-gray-500" />
          </Field>
          <Field label="HCODE">
            <select required disabled={!isCreate}
              value={form.hcode}
              onChange={e => setForm({ ...form, hcode: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50">
              {hcodes.map(h => <option key={h} value={h}>{h}</option>)}
            </select>
          </Field>
          <Field label="License No. (6 หลัก — ใช้เป็น DRDX/DROPID)">
            <input type="text" required maxLength={10}
              value={form.license_no}
              onChange={e => setForm({ ...form, license_no: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono" />
          </Field>
          <Field label="สาขา">
            <input type="text"
              value={form.specialty ?? ''}
              onChange={e => setForm({ ...form, specialty: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm" />
          </Field>
          <Field label="ชื่อ (TH)" span={2}>
            <input type="text"
              value={form.name_th ?? ''}
              onChange={e => setForm({ ...form, name_th: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm" />
          </Field>
          <label className="col-span-2 flex items-center gap-2 text-sm text-gray-700">
            <input type="checkbox" checked={form.is_active}
              onChange={e => setForm({ ...form, is_active: e.target.checked })}
              className="rounded" />
            ใช้งานอยู่
          </label>
        </div>
        <div className="px-5 py-3 border-t border-gray-100 flex items-center justify-end gap-2">
          <button type="button" onClick={onClose}
            className="text-sm px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700">ยกเลิก</button>
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
