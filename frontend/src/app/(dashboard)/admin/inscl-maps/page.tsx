'use client'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { insclMapsApi, hospitalsApi, INSCL_LABELS, type InsclMap, type INSCL } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { Pencil, Trash2, Plus, X } from 'lucide-react'

const INSCL_CODES: INSCL[] = [
  '011', 'WEL', 'LGO', 'OFC',
  'UCS', 'NON', 'WP1', 'WP2',
  'SSS', 'SS4',
  'TPBS', 'WK', 'MON', 'PRS',
]
const AGENCY_CODES = ['', 'CGD', 'LGO', 'NBTC', 'BAAC', 'ECT', 'PEA', 'MEA', 'MWA', 'SRT']

export default function InsclMapsPage() {
  const qc = useQueryClient()
  const [filterHcode, setFilterHcode] = useState('')

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })
  const list = useQuery({
    queryKey: ['inscl-maps', filterHcode],
    queryFn: () => insclMapsApi.list(filterHcode || undefined),
  })

  const [editing, setEditing] = useState<InsclMap | null>(null)
  const [creating, setCreating] = useState(false)

  const save = useMutation({
    mutationFn: (m: InsclMap) => insclMapsApi.upsert(m),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['inscl-maps'] })
      setEditing(null); setCreating(false)
    },
  })
  const remove = useMutation({
    mutationFn: (key: { hcode: string; hisPttype: string }) => insclMapsApi.delete(key.hcode, key.hisPttype),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['inscl-maps'] }),
  })

  if (list.isLoading) return <LoadingBlock />
  if (list.isError) return <ErrorBlock error={list.error} onRetry={() => list.refetch()} />

  const rows = list.data?.items ?? []
  const hcodes = hospitals.data?.hospitals.map(h => h.hcode) ?? []

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        แปลงรหัส HIS PTTYPE → INSCL มาตรฐาน + Agency (สำหรับ OFC). Pipeline ใช้ตารางนี้ route ให้ถูก format/sender.
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
        </div>
        <button
          onClick={() => {
            setCreating(true)
            setEditing({ hcode: hcodes[0] ?? '', his_pttype: '', inscl: 'UCS' })
          }}
          disabled={hcodes.length === 0}
          className="text-xs px-3 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50 inline-flex items-center gap-1.5"
        >
          <Plus className="w-3.5 h-3.5" /> เพิ่ม mapping
        </button>
      </div>
      {hcodes.length === 0 && (
        <p className="text-xs text-amber-700 bg-amber-50 border border-amber-100 rounded px-3 py-2">
          ยังไม่มีรายการโรงพยาบาล — เพิ่มที่ /admin/hospitals ก่อน
        </p>
      )}

      {rows.length === 0 ? (
        <EmptyBlock label="ยังไม่มี mapping — กด 'เพิ่ม mapping' เพื่อเริ่ม" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">HCODE</th>
                <th className="px-4 py-2.5 text-left font-medium">HIS PTTYPE</th>
                <th className="px-4 py-2.5 text-left font-medium">→ INSCL</th>
                <th className="px-4 py-2.5 text-left font-medium">Agency</th>
                <th className="px-4 py-2.5 text-left font-medium">Note</th>
                <th className="px-4 py-2.5 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(m => (
                <tr key={m.hcode + '|' + m.his_pttype} className="hover:bg-gray-50">
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-600">{m.hcode}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-700">{m.his_pttype}</td>
                  <td className="px-4 py-2.5">
                    <code className="bg-gray-100 px-1.5 py-0.5 rounded text-xs">{m.inscl}</code>
                    <span className="text-xs text-gray-400 ml-2">
                      {INSCL_LABELS[m.inscl as INSCL] ?? ''}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-500">{m.agency_code || '—'}</td>
                  <td className="px-4 py-2.5 text-xs text-gray-500 max-w-xs truncate">{m.note || '—'}</td>
                  <td className="px-4 py-2.5 text-right space-x-1">
                    <button onClick={() => { setCreating(false); setEditing(m) }}
                      className="p-1.5 rounded hover:bg-gray-100 text-gray-600">
                      <Pencil className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={() => {
                        if (confirm(`ลบ mapping ${m.hcode} / ${m.his_pttype}?`)) {
                          remove.mutate({ hcode: m.hcode, hisPttype: m.his_pttype })
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
        <MapDialog
          initial={editing} isCreate={creating} hcodes={hcodes} busy={save.isPending}
          onClose={() => { setEditing(null); setCreating(false) }}
          onSubmit={m => save.mutate(m)}
        />
      )}
    </div>
  )
}

function MapDialog({
  initial, isCreate, hcodes, busy, onClose, onSubmit,
}: {
  initial: InsclMap; isCreate: boolean; hcodes: string[]; busy: boolean
  onClose: () => void; onSubmit: (m: InsclMap) => void
}) {
  const [form, setForm] = useState<InsclMap>(initial)
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <form onSubmit={e => { e.preventDefault(); onSubmit(form) }}
        className="bg-white rounded-xl w-[520px] max-w-[92vw] shadow-xl">
        <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
          <h2 className="text-sm font-medium text-gray-900">
            {isCreate ? 'เพิ่ม mapping' : `แก้ไข ${initial.hcode} / ${initial.his_pttype}`}
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
          <Field label="HIS PTTYPE (ใน HIS)">
            <input type="text" required disabled={!isCreate}
              value={form.his_pttype}
              onChange={e => setForm({ ...form, his_pttype: e.target.value })}
              placeholder="เช่น A01, GOV"
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono disabled:bg-gray-50" />
          </Field>
          <Field label="→ INSCL มาตรฐาน">
            <select required
              value={form.inscl}
              onChange={e => setForm({ ...form, inscl: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm">
              {INSCL_CODES.map(c => (
                <option key={c} value={c}>{c} — {INSCL_LABELS[c]}</option>
              ))}
            </select>
          </Field>
          <Field label="Agency (เฉพาะ OFC)">
            <select
              value={form.agency_code ?? ''}
              onChange={e => setForm({ ...form, agency_code: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono">
              {AGENCY_CODES.map(a => <option key={a} value={a}>{a || '(none)'}</option>)}
            </select>
          </Field>
          <Field label="Note" span={2}>
            <input type="text"
              value={form.note ?? ''}
              onChange={e => setForm({ ...form, note: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm" />
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
