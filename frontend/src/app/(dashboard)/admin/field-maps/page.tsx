'use client'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { fieldMapsApi, hospitalsApi, type FieldMap } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { BulkImportModal } from '@/components/BulkImportModal'
import { Pencil, Trash2, Plus, X, Upload } from 'lucide-react'

const TARGET_FILES = ['OPD', 'IPD', 'ODX', 'OOP', 'IDX', 'IOP', 'DRU', 'CHT', 'CHA', 'AER', 'ADP', 'INS', 'PAT', 'LVD', 'IRF', 'ORF']
const TRANSFORMS = [
  'none', 'to_ad', 'format_hhmm', 'format_yyyymmdd',
  'map_inscl', 'map_drug_tmt', 'map_doctor_license',
  'validate_icd10', 'validate_icd9cm', 'validate_tmt24',
  'pad_left', 'uppercase',
]

export default function FieldMapsPage() {
  const qc = useQueryClient()
  const [filterHcode, setFilterHcode] = useState('')
  const [filterHISTable, setFilterHISTable] = useState('')
  const [filterTargetFile, setFilterTargetFile] = useState('')

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })
  const list = useQuery({
    queryKey: ['field-maps', filterHcode, filterHISTable, filterTargetFile],
    queryFn: () => fieldMapsApi.list({
      hcode:       filterHcode || undefined,
      his_table:   filterHISTable || undefined,
      target_file: filterTargetFile || undefined,
    }),
  })

  const [editing, setEditing] = useState<FieldMap | null>(null)
  const [creating, setCreating] = useState(false)
  const [bulkOpen, setBulkOpen] = useState(false)

  const save = useMutation({
    mutationFn: (m: FieldMap) => fieldMapsApi.upsert(m),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['field-maps'] })
      setEditing(null); setCreating(false)
    },
  })
  const remove = useMutation({
    mutationFn: (id: string) => fieldMapsApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['field-maps'] }),
  })

  if (list.isLoading) return <LoadingBlock />
  if (list.isError) return <ErrorBlock error={list.error} onRetry={() => list.refetch()} />

  const rows = list.data?.items ?? []
  const hcodes = hospitals.data?.hospitals.map(h => h.hcode) ?? []

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        จับคู่ HIS column (table.column) → target field ในแฟ้มส่ง (OPD/IPD/DRU/...) พร้อม
        transform function. ขนาดทั่วไป 100–200 mapping ต่อ รพ. — แนะนำ bulk import.
      </p>

      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-2 flex-wrap">
          <select value={filterHcode}
            onChange={e => setFilterHcode(e.target.value)}
            className="text-xs border border-gray-200 rounded-lg px-2 py-1">
            <option value="">ทั้ง รพ.</option>
            {hcodes.map(h => <option key={h} value={h}>{h}</option>)}
          </select>
          <input type="text" value={filterHISTable}
            onChange={e => setFilterHISTable(e.target.value)}
            placeholder="HIS table เช่น opd_visit"
            className="text-xs border border-gray-200 rounded-lg px-2 py-1 font-mono w-40" />
          <select value={filterTargetFile}
            onChange={e => setFilterTargetFile(e.target.value)}
            className="text-xs border border-gray-200 rounded-lg px-2 py-1">
            <option value="">ทุกแฟ้ม</option>
            {TARGET_FILES.map(f => <option key={f} value={f}>{f}</option>)}
          </select>
          <span className="text-xs text-gray-400">{rows.length} รายการ</span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setBulkOpen(true)}
            disabled={hcodes.length === 0}
            className="text-xs px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700 disabled:opacity-50 inline-flex items-center gap-1.5"
          >
            <Upload className="w-3.5 h-3.5" /> Bulk import
          </button>
          <button
            onClick={() => {
              setCreating(true)
              setEditing({
                hcode: hcodes[0] ?? '', his_table: '', his_column: '',
                target_file: 'OPD', target_field: '',
                transform: 'none', is_required: false,
              })
            }}
            disabled={hcodes.length === 0}
            className="text-xs px-3 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50 inline-flex items-center gap-1.5"
          >
            <Plus className="w-3.5 h-3.5" /> เพิ่ม mapping
          </button>
        </div>
      </div>

      {rows.length === 0 ? (
        <EmptyBlock label="ยังไม่มี field mapping — ใช้ bulk import เพื่อ seed เร็วๆ" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-3 py-2.5 text-left font-medium">HCODE</th>
                <th className="px-3 py-2.5 text-left font-medium">HIS table.column</th>
                <th className="px-3 py-2.5 text-left font-medium">→ File.Field</th>
                <th className="px-3 py-2.5 text-left font-medium">Transform</th>
                <th className="px-3 py-2.5 text-left font-medium">Req</th>
                <th className="px-3 py-2.5 text-left font-medium">Default</th>
                <th className="px-3 py-2.5 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(m => (
                <tr key={m.id} className="hover:bg-gray-50">
                  <td className="px-3 py-2 font-mono text-xs text-gray-600">{m.hcode}</td>
                  <td className="px-3 py-2 font-mono text-xs text-gray-700">
                    {m.his_table}.<span className="text-gray-900">{m.his_column}</span>
                  </td>
                  <td className="px-3 py-2 font-mono text-xs">
                    <span className="bg-gray-100 px-1.5 py-0.5 rounded">{m.target_file}</span>
                    .<span className="text-primary-600 font-medium">{m.target_field}</span>
                  </td>
                  <td className="px-3 py-2 font-mono text-[11px] text-gray-500">{m.transform || 'none'}</td>
                  <td className="px-3 py-2">
                    {m.is_required
                      ? <span className="text-[11px] px-1.5 py-0.5 bg-amber-50 text-amber-700 rounded">required</span>
                      : <span className="text-[11px] text-gray-400">—</span>}
                  </td>
                  <td className="px-3 py-2 font-mono text-[11px] text-gray-500">{m.default_value || '—'}</td>
                  <td className="px-3 py-2 text-right space-x-1">
                    <button onClick={() => { setCreating(false); setEditing(m) }}
                      className="p-1.5 rounded hover:bg-gray-100 text-gray-600">
                      <Pencil className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={() => {
                        if (!m.id) return
                        if (confirm(`ลบ mapping ${m.his_table}.${m.his_column} → ${m.target_file}.${m.target_field}?`)) {
                          remove.mutate(m.id)
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
        <BulkImportModal<FieldMap>
          title="Bulk import field mappings"
          required={['hcode', 'his_table', 'his_column', 'target_file', 'target_field']}
          sampleHint="columns อื่น ๆ: transform (none/to_ad/...), is_required (true/false), default_value, note"
          toItem={r => ({
            hcode:         r.hcode,
            his_table:     r.his_table,
            his_column:    r.his_column,
            target_file:   r.target_file,
            target_field:  r.target_field,
            transform:     r.transform || undefined,
            is_required:   (r.is_required ?? 'false').toLowerCase() === 'true',
            default_value: r.default_value || undefined,
            note:          r.note || undefined,
          })}
          onSubmit={items => fieldMapsApi.bulk(items)}
          onClose={() => setBulkOpen(false)}
          onDone={() => qc.invalidateQueries({ queryKey: ['field-maps'] })}
        />
      )}
    </div>
  )
}

function Dialog({
  initial, isCreate, hcodes, busy, onClose, onSubmit,
}: {
  initial: FieldMap; isCreate: boolean; hcodes: string[]; busy: boolean
  onClose: () => void; onSubmit: (m: FieldMap) => void
}) {
  const [form, setForm] = useState<FieldMap>(initial)
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <form onSubmit={e => { e.preventDefault(); onSubmit(form) }}
        className="bg-white rounded-xl w-[640px] max-w-[92vw] shadow-xl">
        <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
          <h2 className="text-sm font-medium text-gray-900">
            {isCreate ? 'เพิ่ม field mapping' : `แก้ไข mapping`}
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
          <Field label="Target File">
            <select required disabled={!isCreate}
              value={form.target_file}
              onChange={e => setForm({ ...form, target_file: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50">
              {TARGET_FILES.map(f => <option key={f} value={f}>{f}</option>)}
            </select>
          </Field>
          <Field label="HIS Table">
            <input type="text" required disabled={!isCreate}
              value={form.his_table}
              onChange={e => setForm({ ...form, his_table: e.target.value })}
              placeholder="opd_visit"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50" />
          </Field>
          <Field label="HIS Column">
            <input type="text" required disabled={!isCreate}
              value={form.his_column}
              onChange={e => setForm({ ...form, his_column: e.target.value })}
              placeholder="vstdate"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50" />
          </Field>
          <Field label="→ Target Field" span={2}>
            <input type="text" required disabled={!isCreate}
              value={form.target_field}
              onChange={e => setForm({ ...form, target_field: e.target.value })}
              placeholder="DATEOPD"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50" />
          </Field>
          <Field label="Transform">
            <select
              value={form.transform ?? 'none'}
              onChange={e => setForm({ ...form, transform: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono">
              {TRANSFORMS.map(t => <option key={t} value={t}>{t}</option>)}
            </select>
          </Field>
          <Field label="Default Value">
            <input type="text"
              value={form.default_value ?? ''}
              onChange={e => setForm({ ...form, default_value: e.target.value })}
              placeholder='เช่น "1" สำหรับ UUC'
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono" />
          </Field>
          <Field label="Note" span={2}>
            <input type="text"
              value={form.note ?? ''}
              onChange={e => setForm({ ...form, note: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm" />
          </Field>
          <label className="col-span-2 flex items-center gap-2 text-sm text-gray-700">
            <input type="checkbox" checked={form.is_required}
              onChange={e => setForm({ ...form, is_required: e.target.checked })}
              className="rounded" />
            Required (ส่ง record ไม่ได้ถ้า field นี้ว่าง)
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
