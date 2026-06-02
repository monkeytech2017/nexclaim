'use client'
import { useEffect, useRef, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { drugMapsApi, hospitalsApi, tmtApi, type DrugMap, type TmtDrug } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { BulkImportModal } from '@/components/BulkImportModal'
import { Pencil, Trash2, Plus, X, Upload, Search, Loader2 } from 'lucide-react'

export default function DrugMapsPage() {
  const qc = useQueryClient()
  const [filterHcode, setFilterHcode] = useState('')

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })
  const list = useQuery({
    queryKey: ['drug-maps', filterHcode],
    queryFn: () => drugMapsApi.list(filterHcode || undefined),
  })

  const [editing, setEditing] = useState<DrugMap | null>(null)
  const [creating, setCreating] = useState(false)
  const [bulkOpen, setBulkOpen] = useState(false)

  const save = useMutation({
    mutationFn: (m: DrugMap) => drugMapsApi.upsert(m),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['drug-maps'] })
      setEditing(null); setCreating(false)
    },
  })
  const remove = useMutation({
    mutationFn: (k: { hcode: string; code: string }) => drugMapsApi.delete(k.hcode, k.code),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['drug-maps'] }),
  })

  if (list.isLoading) return <LoadingBlock />
  if (list.isError) return <ErrorBlock error={list.error} onRetry={() => list.refetch()} />

  const rows = list.data?.items ?? []
  const hcodes = hospitals.data?.hospitals.map(h => h.hcode) ?? []

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        แปลงรหัสยาใน HIS → TMT 24 หลัก. mapping ต่อ รพ. — pipeline ใช้ตารางนี้หา TMT ให้ drug ใน DRU.txt/DRUG XML.
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
              setEditing({ hcode: hcodes[0] ?? '', his_drug_code: '', is_active: true })
            }}
            disabled={hcodes.length === 0}
            className="text-xs px-3 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50 inline-flex items-center gap-1.5"
          >
            <Plus className="w-3.5 h-3.5" /> เพิ่ม mapping
          </button>
        </div>
      </div>

      {rows.length === 0 ? (
        <EmptyBlock label="ยังไม่มี drug mapping — Bulk CSV import จะมาใน slice ถัดไป" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">HCODE</th>
                <th className="px-4 py-2.5 text-left font-medium">HIS Drug Code</th>
                <th className="px-4 py-2.5 text-left font-medium">HIS Drug Name</th>
                <th className="px-4 py-2.5 text-left font-medium">→ TMT24</th>
                <th className="px-4 py-2.5 text-left font-medium">สถานะ</th>
                <th className="px-4 py-2.5 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(m => (
                <tr key={m.hcode + '|' + m.his_drug_code} className="hover:bg-gray-50">
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-600">{m.hcode}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-700">{m.his_drug_code}</td>
                  <td className="px-4 py-2.5 text-gray-900">{m.his_drug_name || '—'}</td>
                  <td className="px-4 py-2.5 font-mono text-[11px] text-gray-500">
                    {m.tmt_code || <span className="text-amber-600">unmapped</span>}
                  </td>
                  <td className="px-4 py-2.5">
                    <span className={`text-[11px] px-2 py-0.5 rounded-full font-medium ${
                      m.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'
                    }`}>
                      {m.is_active ? 'ใช้งาน' : 'ปิด'}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-right space-x-1">
                    <button onClick={() => { setCreating(false); setEditing(m) }}
                      className="p-1.5 rounded hover:bg-gray-100 text-gray-600">
                      <Pencil className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={() => {
                        if (confirm(`ลบ drug mapping ${m.hcode} / ${m.his_drug_code}?`)) {
                          remove.mutate({ hcode: m.hcode, code: m.his_drug_code })
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
        <BulkImportModal<DrugMap>
          title="Bulk import drug mappings"
          required={['hcode', 'his_drug_code']}
          sampleHint="columns อื่น ๆ: tmt_code (24 หลัก ถ้ามี), his_drug_name, note, is_active (true/false)"
          toItem={r => ({
            hcode: r.hcode,
            his_drug_code: r.his_drug_code,
            tmt_code: r.tmt_code || undefined,
            his_drug_name: r.his_drug_name || undefined,
            note: r.note || undefined,
            is_active: (r.is_active ?? 'true').toLowerCase() !== 'false',
          })}
          onSubmit={items => drugMapsApi.bulk(items)}
          onClose={() => setBulkOpen(false)}
          onDone={() => qc.invalidateQueries({ queryKey: ['drug-maps'] })}
        />
      )}
    </div>
  )
}

function Dialog({
  initial, isCreate, hcodes, busy, onClose, onSubmit,
}: {
  initial: DrugMap; isCreate: boolean; hcodes: string[]; busy: boolean
  onClose: () => void; onSubmit: (m: DrugMap) => void
}) {
  const [form, setForm] = useState<DrugMap>(initial)
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <form onSubmit={e => { e.preventDefault(); onSubmit(form) }}
        className="bg-white rounded-xl w-[560px] max-w-[92vw] shadow-xl">
        <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
          <h2 className="text-sm font-medium text-gray-900">
            {isCreate ? 'เพิ่ม drug mapping' : `แก้ไข ${initial.hcode} / ${initial.his_drug_code}`}
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
          <Field label="HIS Drug Code">
            <input type="text" required disabled={!isCreate}
              value={form.his_drug_code}
              onChange={e => setForm({ ...form, his_drug_code: e.target.value })}
              placeholder="เช่น DRG000124"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50" />
          </Field>
          <Field label="HIS Drug Name" span={2}>
            <input type="text"
              value={form.his_drug_name ?? ''}
              onChange={e => setForm({ ...form, his_drug_name: e.target.value })}
              placeholder="เช่น AMLODIPINE 5MG TAB"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm" />
          </Field>
          <Field label="ค้นหายา TMT (พิมพ์ชื่อ → เลือก)" span={2}>
            <TmtPicker
              value={form.tmt_code ?? ''}
              onChange={code => setForm({ ...form, tmt_code: code })}
            />
          </Field>
          <Field label="TMT24 (24 หลัก)" span={2}>
            <input type="text" maxLength={24}
              value={form.tmt_code ?? ''}
              onChange={e => setForm({ ...form, tmt_code: e.target.value })}
              placeholder="100452100101010100000612"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono" />
          </Field>
          <Field label="Note" span={2}>
            <input type="text"
              value={form.note ?? ''}
              onChange={e => setForm({ ...form, note: e.target.value })}
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

// Inline typeahead: search TMT master by name, pick → sets tmt_code on the form.
// Self-contained; the raw tmt_code input stays editable for manual entry.
function TmtPicker({ value, onChange }: { value: string; onChange: (code: string) => void }) {
  const [input, setInput] = useState('')
  const [q, setQ] = useState('')
  const [open, setOpen] = useState(false)
  const [picked, setPicked] = useState<TmtDrug | null>(null)
  const boxRef = useRef<HTMLDivElement>(null)

  // debounce ~300ms: input → q
  useEffect(() => {
    const t = setTimeout(() => setQ(input.trim()), 300)
    return () => clearTimeout(t)
  }, [input])

  // close dropdown on outside click
  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [])

  // clear the confirmation if the form's code no longer matches the picked drug
  useEffect(() => {
    if (picked && picked.tmt_code !== value) setPicked(null)
  }, [value, picked])

  const search = useQuery({
    queryKey: ['tmt-picker', q],
    queryFn: () => tmtApi.search({ q, limit: 8 }),
    enabled: q.length >= 2,
  })
  const results = search.data?.items ?? []

  return (
    <div ref={boxRef} className="relative">
      <div className="relative">
        <Search className="w-3.5 h-3.5 text-gray-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
        <input
          type="text"
          value={input}
          onChange={e => { setInput(e.target.value); setOpen(true) }}
          onFocus={() => setOpen(true)}
          placeholder="พิมพ์ชื่อยา เช่น AMLODIPINE..."
          className="w-full border border-gray-200 rounded-lg pl-8 pr-3 py-2 text-sm"
        />
        {search.isFetching && (
          <Loader2 className="w-3.5 h-3.5 text-gray-400 animate-spin absolute right-2.5 top-1/2 -translate-y-1/2" />
        )}
      </div>

      {open && q.length >= 2 && (
        <div className="absolute z-10 mt-1 w-full bg-white border border-gray-200 rounded-lg shadow-lg max-h-64 overflow-auto">
          {search.isError ? (
            <div className="px-3 py-2 text-xs text-red-600">ค้นหาไม่สำเร็จ</div>
          ) : results.length === 0 && !search.isFetching ? (
            <div className="px-3 py-2 text-xs text-gray-400">ไม่พบยา</div>
          ) : (
            results.map(d => (
              <button
                key={d.tmt_code}
                type="button"
                onClick={() => { onChange(d.tmt_code); setPicked(d); setOpen(false); setInput('') }}
                className="w-full text-left px-3 py-2 hover:bg-gray-50 border-b border-gray-50 last:border-0"
              >
                <div className="text-sm text-gray-900">{d.name_th || d.generic_name || '—'}</div>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-[11px] text-gray-500">{d.tmt_code}</span>
                  {d.strength && <span className="text-[11px] text-gray-400">{d.strength}</span>}
                  {d.dosage_form && <span className="text-[11px] text-gray-400">{d.dosage_form}</span>}
                </div>
              </button>
            ))
          )}
        </div>
      )}

      {picked && (
        <div className="mt-1.5 text-xs text-green-700 bg-green-50 rounded-lg px-2.5 py-1.5">
          เลือก: <span className="font-medium">{picked.name_th || picked.generic_name}</span>
          <span className="font-mono text-[11px] text-green-600 ml-2">{picked.tmt_code}</span>
        </div>
      )}
    </div>
  )
}
