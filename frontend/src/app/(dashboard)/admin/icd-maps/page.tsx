'use client'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { icdMapsApi, hospitalsApi, type IcdMap, type IcdType } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { BulkImportModal } from '@/components/BulkImportModal'
import { Pencil, Trash2, Plus, X, Upload } from 'lucide-react'

export default function IcdMapsPage() {
  const qc = useQueryClient()
  const [filterHcode, setFilterHcode] = useState('')
  const [filterType, setFilterType] = useState<'' | IcdType>('')

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })
  const list = useQuery({
    queryKey: ['icd-maps', filterHcode, filterType],
    queryFn: () => icdMapsApi.list(filterHcode || undefined, (filterType || undefined) as IcdType | undefined),
  })

  const [editing, setEditing] = useState<IcdMap | null>(null)
  const [creating, setCreating] = useState(false)
  const [bulkOpen, setBulkOpen] = useState(false)

  const save = useMutation({
    mutationFn: (m: IcdMap) => icdMapsApi.upsert(m),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['icd-maps'] })
      setEditing(null); setCreating(false)
    },
  })
  const remove = useMutation({
    mutationFn: (k: { hcode: string; type: IcdType; code: string }) =>
      icdMapsApi.delete(k.hcode, k.type, k.code),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['icd-maps'] }),
  })

  if (list.isLoading) return <LoadingBlock />
  if (list.isError) return <ErrorBlock error={list.error} onRetry={() => list.refetch()} />

  const rows = list.data?.items ?? []
  const hcodes = hospitals.data?.hospitals.map(h => h.hcode) ?? []

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        แปลงรหัส ICD ภายในของ HIS → รหัสมาตรฐาน (ICD-10 WHO หรือ ICD-9CM).
        ใช้เฉพาะกรณี HIS ใช้รหัสที่ไม่ตรง master — ปกติส่วนใหญ่ใช้ตรงได้ ไม่ต้อง map.
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
          <span className="text-xs text-gray-500 ml-2">ประเภท:</span>
          <select value={filterType}
            onChange={e => setFilterType(e.target.value as '' | IcdType)}
            className="text-xs border border-gray-200 rounded-lg px-2 py-1">
            <option value="">ทั้งหมด</option>
            <option value="10">ICD-10</option>
            <option value="9C">ICD-9CM</option>
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
              setEditing({ hcode: hcodes[0] ?? '', his_icd_code: '', icd_type: '10', std_code: '' })
            }}
            disabled={hcodes.length === 0}
            className="text-xs px-3 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50 inline-flex items-center gap-1.5"
          >
            <Plus className="w-3.5 h-3.5" /> เพิ่ม mapping
          </button>
        </div>
      </div>

      {rows.length === 0 ? (
        <EmptyBlock label="ยังไม่มี ICD mapping" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">HCODE</th>
                <th className="px-4 py-2.5 text-left font-medium">ประเภท</th>
                <th className="px-4 py-2.5 text-left font-medium">HIS ICD Code</th>
                <th className="px-4 py-2.5 text-left font-medium">→ Standard Code</th>
                <th className="px-4 py-2.5 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(m => (
                <tr key={m.hcode + '|' + m.icd_type + '|' + m.his_icd_code} className="hover:bg-gray-50">
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-600">{m.hcode}</td>
                  <td className="px-4 py-2.5">
                    <span className="text-[11px] bg-gray-100 px-2 py-0.5 rounded font-mono">
                      {m.icd_type === '10' ? 'ICD-10' : 'ICD-9CM'}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-700">{m.his_icd_code}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-900">{m.std_code}</td>
                  <td className="px-4 py-2.5 text-right space-x-1">
                    <button onClick={() => { setCreating(false); setEditing(m) }}
                      className="p-1.5 rounded hover:bg-gray-100 text-gray-600">
                      <Pencil className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={() => {
                        if (confirm(`ลบ ICD mapping ${m.hcode} / ${m.icd_type} / ${m.his_icd_code}?`)) {
                          remove.mutate({ hcode: m.hcode, type: m.icd_type, code: m.his_icd_code })
                        }
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
        <Dialog initial={editing} isCreate={creating} hcodes={hcodes} busy={save.isPending}
          onClose={() => { setEditing(null); setCreating(false) }}
          onSubmit={m => save.mutate(m)} />
      )}

      {bulkOpen && (
        <BulkImportModal<IcdMap>
          title="Bulk import ICD mappings"
          required={['hcode', 'his_icd_code', 'icd_type', 'std_code']}
          sampleHint="icd_type ต้องเป็น '10' (ICD-10) หรือ '9C' (ICD-9CM)"
          toItem={r => ({
            hcode: r.hcode,
            his_icd_code: r.his_icd_code,
            icd_type: (r.icd_type === '9C' ? '9C' : '10') as IcdType,
            std_code: r.std_code,
          })}
          onSubmit={items => icdMapsApi.bulk(items)}
          onClose={() => setBulkOpen(false)}
          onDone={() => qc.invalidateQueries({ queryKey: ['icd-maps'] })}
        />
      )}
    </div>
  )
}

function Dialog({
  initial, isCreate, hcodes, busy, onClose, onSubmit,
}: {
  initial: IcdMap; isCreate: boolean; hcodes: string[]; busy: boolean
  onClose: () => void; onSubmit: (m: IcdMap) => void
}) {
  const [form, setForm] = useState<IcdMap>(initial)
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <form onSubmit={e => { e.preventDefault(); onSubmit(form) }}
        className="bg-white rounded-xl w-[520px] max-w-[92vw] shadow-xl">
        <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
          <h2 className="text-sm font-medium text-gray-900">
            {isCreate ? 'เพิ่ม ICD mapping' : `แก้ไข ${initial.hcode} / ${initial.icd_type} / ${initial.his_icd_code}`}
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
          <Field label="ประเภท">
            <select required disabled={!isCreate}
              value={form.icd_type}
              onChange={e => setForm({ ...form, icd_type: e.target.value as IcdType })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm disabled:bg-gray-50">
              <option value="10">ICD-10</option>
              <option value="9C">ICD-9CM</option>
            </select>
          </Field>
          <Field label="HIS ICD Code">
            <input type="text" required disabled={!isCreate}
              value={form.his_icd_code}
              onChange={e => setForm({ ...form, his_icd_code: e.target.value })}
              placeholder="รหัสใน HIS"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50" />
          </Field>
          <Field label="→ Standard Code">
            <input type="text" required maxLength={7}
              value={form.std_code}
              onChange={e => setForm({ ...form, std_code: e.target.value })}
              placeholder={form.icd_type === '10' ? 'เช่น I10, E11.9' : 'เช่น 47.0'}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono" />
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

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-xs text-gray-500 mb-1">{label}</label>
      {children}
    </div>
  )
}
