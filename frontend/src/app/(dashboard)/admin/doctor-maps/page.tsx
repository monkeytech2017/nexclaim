'use client'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { doctorMapsApi, doctorsApi, hospitalsApi, type DoctorMap } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { BulkImportModal } from '@/components/BulkImportModal'
import { Pencil, Trash2, Plus, X, Upload } from 'lucide-react'

export default function DoctorMapsPage() {
  const qc = useQueryClient()
  const [filterHcode, setFilterHcode] = useState('')

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })
  const doctors = useQuery({
    queryKey: ['doctors', filterHcode || '__all'],
    queryFn: () => doctorsApi.list(filterHcode || undefined),
  })
  const list = useQuery({
    queryKey: ['doctor-maps', filterHcode],
    queryFn: () => doctorMapsApi.list(filterHcode || undefined),
  })

  const [editing, setEditing] = useState<DoctorMap | null>(null)
  const [creating, setCreating] = useState(false)
  const [bulkOpen, setBulkOpen] = useState(false)

  const save = useMutation({
    mutationFn: (m: DoctorMap) => doctorMapsApi.upsert(m),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['doctor-maps'] })
      setEditing(null); setCreating(false)
    },
  })
  const remove = useMutation({
    mutationFn: (k: { hcode: string; code: string }) => doctorMapsApi.delete(k.hcode, k.code),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['doctor-maps'] }),
  })

  if (list.isLoading) return <LoadingBlock />
  if (list.isError) return <ErrorBlock error={list.error} onRetry={() => list.refetch()} />

  const rows = list.data?.items ?? []
  const hcodes = hospitals.data?.hospitals.map(h => h.hcode) ?? []
  const doctorList = doctors.data?.doctors ?? []

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        แปลงรหัสแพทย์ใน HIS → m_doctor.doctor_id (license_no ที่ใช้เป็น DRDX/DROPID).
      </p>
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500">โรงพยาบาล:</span>
          <select value={filterHcode}
            onChange={e => setFilterHcode(e.target.value)}
            className="text-xs border border-gray-200 rounded-lg px-2 py-1">
            <option value="">ทั้งหมด</option>
            {hcodes.map(h => <option key={h} value={h}>{h}</option>)}
          </select>
          <span className="text-xs text-gray-400">{rows.length} รายการ</span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setBulkOpen(true)}
            disabled={hcodes.length === 0}
            className="text-xs px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700 disabled:opacity-50 inline-flex items-center gap-1.5"
          >
            <Upload className="w-3.5 h-3.5" /> Bulk import (CSV)
          </button>
          <button
            onClick={() => {
              setCreating(true)
              setEditing({ hcode: hcodes[0] ?? '', his_doctor_code: '' })
            }}
            disabled={hcodes.length === 0}
            className="text-xs px-3 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50 inline-flex items-center gap-1.5"
          >
            <Plus className="w-3.5 h-3.5" /> เพิ่ม mapping
          </button>
        </div>
      </div>

      {rows.length === 0 ? (
        <EmptyBlock label="ยังไม่มี doctor mapping" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">HCODE</th>
                <th className="px-4 py-2.5 text-left font-medium">HIS Doctor Code</th>
                <th className="px-4 py-2.5 text-left font-medium">→ Doctor ID (internal)</th>
                <th className="px-4 py-2.5 text-left font-medium">License (DRDX)</th>
                <th className="px-4 py-2.5 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(m => {
                const d = doctorList.find(x => x.doctor_id === m.doctor_id)
                return (
                  <tr key={m.hcode + '|' + m.his_doctor_code} className="hover:bg-gray-50">
                    <td className="px-4 py-2.5 font-mono text-xs text-gray-600">{m.hcode}</td>
                    <td className="px-4 py-2.5 font-mono text-xs text-gray-700">{m.his_doctor_code}</td>
                    <td className="px-4 py-2.5 font-mono text-xs text-gray-700">
                      {m.doctor_id || <span className="text-amber-600">unmapped</span>}
                    </td>
                    <td className="px-4 py-2.5 font-mono text-xs text-gray-500">
                      {d?.license_no || '—'}
                    </td>
                    <td className="px-4 py-2.5 text-right space-x-1">
                      <button onClick={() => { setCreating(false); setEditing(m) }}
                        className="p-1.5 rounded hover:bg-gray-100 text-gray-600">
                        <Pencil className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => {
                          if (confirm(`ลบ doctor mapping ${m.hcode} / ${m.his_doctor_code}?`)) {
                            remove.mutate({ hcode: m.hcode, code: m.his_doctor_code })
                          }
                        }}
                        className="p-1.5 rounded hover:bg-red-50 text-red-600">
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {save.isError && <ErrorBlock error={save.error} />}
      {remove.isError && <ErrorBlock error={remove.error} />}

      {editing && (
        <Dialog initial={editing} isCreate={creating}
          hcodes={hcodes}
          doctors={doctorList.filter(d => !editing.hcode || d.hcode === editing.hcode)}
          busy={save.isPending}
          onClose={() => { setEditing(null); setCreating(false) }}
          onSubmit={m => save.mutate(m)} />
      )}

      {bulkOpen && (
        <BulkImportModal<DoctorMap>
          title="Bulk import doctor mappings"
          required={['hcode', 'his_doctor_code']}
          sampleHint="columns อื่น ๆ: doctor_id (FK → m_doctor.doctor_id)"
          toItem={r => ({
            hcode: r.hcode,
            his_doctor_code: r.his_doctor_code,
            doctor_id: r.doctor_id || undefined,
          })}
          onSubmit={items => doctorMapsApi.bulk(items)}
          onClose={() => setBulkOpen(false)}
          onDone={() => qc.invalidateQueries({ queryKey: ['doctor-maps'] })}
        />
      )}
    </div>
  )
}

function Dialog({
  initial, isCreate, hcodes, doctors, busy, onClose, onSubmit,
}: {
  initial: DoctorMap; isCreate: boolean
  hcodes: string[]; doctors: { doctor_id: string; name_th?: string; license_no: string }[]
  busy: boolean
  onClose: () => void; onSubmit: (m: DoctorMap) => void
}) {
  const [form, setForm] = useState<DoctorMap>(initial)
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <form onSubmit={e => { e.preventDefault(); onSubmit(form) }}
        className="bg-white rounded-xl w-[520px] max-w-[92vw] shadow-xl">
        <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
          <h2 className="text-sm font-medium text-gray-900">
            {isCreate ? 'เพิ่ม doctor mapping' : `แก้ไข ${initial.hcode} / ${initial.his_doctor_code}`}
          </h2>
          <button type="button" onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="px-5 py-4 grid grid-cols-2 gap-3">
          <Field label="HCODE">
            <select required disabled={!isCreate}
              value={form.hcode}
              onChange={e => setForm({ ...form, hcode: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50">
              {hcodes.map(h => <option key={h} value={h}>{h}</option>)}
            </select>
          </Field>
          <Field label="HIS Doctor Code">
            <input type="text" required disabled={!isCreate}
              value={form.his_doctor_code}
              onChange={e => setForm({ ...form, his_doctor_code: e.target.value })}
              placeholder="เช่น D0045"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50" />
          </Field>
          <Field label="→ Doctor (internal)" span={2}>
            <select
              value={form.doctor_id ?? ''}
              onChange={e => setForm({ ...form, doctor_id: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm">
              <option value="">(unmapped)</option>
              {doctors.map(d => (
                <option key={d.doctor_id} value={d.doctor_id}>
                  {d.doctor_id} — {d.name_th || '—'} (license {d.license_no})
                </option>
              ))}
            </select>
          </Field>
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
