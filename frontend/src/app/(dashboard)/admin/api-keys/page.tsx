'use client'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiKeysApi, hospitalsApi, type APIKey, type APIKeyCreateResponse } from '@/lib/api'
import { LoadingBlock, ErrorBlock, EmptyBlock } from '@/components/ui/feedback'
import { RoleBadge } from '@/components/ui/badges'
import { Plus, RefreshCw, X, Copy, Check, AlertTriangle } from 'lucide-react'

type ActiveFilter = '' | 'active' | 'inactive'

export default function ApiKeysPage() {
  const qc = useQueryClient()
  const [activeFilter, setActiveFilter] = useState<ActiveFilter>('')
  const [creating, setCreating] = useState(false)

  const list = useQuery({
    queryKey: ['api-keys', activeFilter],
    queryFn: apiKeysApi.list,
    refetchInterval: 30_000,
  })

  const setActive = useMutation({
    mutationFn: ({ id, is_active }: { id: string; is_active: boolean }) =>
      apiKeysApi.setActive(id, is_active),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['api-keys'] }),
  })

  if (list.isLoading) return <LoadingBlock />
  if (list.isError) return <ErrorBlock error={list.error} onRetry={() => list.refetch()} />

  const allRows = list.data?.items ?? []
  const rows = allRows.filter(k => {
    if (activeFilter === 'active')   return k.is_active
    if (activeFilter === 'inactive') return !k.is_active
    return true
  })

  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        จัดการ API key สำหรับเข้าใช้งาน NexClaim — ทั้ง admin และ key เฉพาะ รพ.
      </p>

      <div className="flex items-center gap-2 flex-wrap">
        <select
          value={activeFilter}
          onChange={e => setActiveFilter(e.target.value as ActiveFilter)}
          className="text-xs border border-gray-200 rounded-lg px-2 py-1"
        >
          <option value="">ทั้งหมด</option>
          <option value="active">ใช้งานอยู่</option>
          <option value="inactive">ปิดใช้งาน</option>
        </select>
        <span className="text-xs text-gray-400">{rows.length} รายการ</span>
        <button
          onClick={() => list.refetch()}
          className="text-xs px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-600 inline-flex items-center gap-1.5"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${list.isFetching ? 'animate-spin' : ''}`} /> รีเฟรช
        </button>
        <button
          onClick={() => setCreating(true)}
          className="ml-auto text-xs px-3 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 inline-flex items-center gap-1.5"
        >
          <Plus className="w-3.5 h-3.5" /> สร้าง key ใหม่
        </button>
      </div>

      {setActive.isError && <ErrorBlock error={setActive.error} />}

      {rows.length === 0 ? (
        <EmptyBlock label="ยังไม่มี API key — สร้าง key แรกได้เลย" />
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50 text-xs text-gray-500">
                <th className="px-4 py-2.5 text-left font-medium">Name</th>
                <th className="px-4 py-2.5 text-left font-medium">Role</th>
                <th className="px-4 py-2.5 text-left font-medium">HCODE</th>
                <th className="px-4 py-2.5 text-left font-medium">สร้างเมื่อ</th>
                <th className="px-4 py-2.5 text-left font-medium">ใช้งานล่าสุด</th>
                <th className="px-4 py-2.5 text-left font-medium">สถานะ</th>
                <th className="px-4 py-2.5 text-right font-medium">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {rows.map(k => (
                <tr key={k.id} className="hover:bg-gray-50">
                  <td className="px-4 py-2.5 text-gray-900">{k.name}</td>
                  <td className="px-4 py-2.5"><RoleBadge role={k.role} /></td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-700">{k.hcode || '—'}</td>
                  <td className="px-4 py-2.5 text-xs text-gray-500">{formatDT(k.created_at)}</td>
                  <td className="px-4 py-2.5 text-xs text-gray-500">
                    {k.last_used_at ? formatDT(k.last_used_at) : <span className="text-gray-300">—</span>}
                  </td>
                  <td className="px-4 py-2.5">
                    <span className={`text-[11px] px-2 py-0.5 rounded-full font-medium ${
                      k.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'
                    }`}>
                      {k.is_active ? 'active' : 'inactive'}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    {k.is_active ? (
                      <button
                        disabled={setActive.isPending}
                        onClick={() => {
                          if (window.confirm('ปิดใช้งาน key นี้? คนที่ถือ key จะใช้ต่อไม่ได้')) {
                            setActive.mutate({ id: k.id, is_active: false })
                          }
                        }}
                        className="text-xs px-2.5 py-1 rounded-lg border border-red-200 text-red-700 hover:bg-red-50 disabled:opacity-50"
                      >
                        ปิดใช้งาน
                      </button>
                    ) : (
                      <button
                        disabled={setActive.isPending}
                        onClick={() => setActive.mutate({ id: k.id, is_active: true })}
                        className="text-xs px-2.5 py-1 rounded-lg border border-gray-200 text-gray-700 hover:bg-gray-50 disabled:opacity-50"
                      >
                        เปิดใช้งาน
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {creating && (
        <CreateKeyModal
          onClose={() => {
            setCreating(false)
            qc.invalidateQueries({ queryKey: ['api-keys'] })
          }}
        />
      )}
    </div>
  )
}

function CreateKeyModal({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient()
  const [role, setRole] = useState<'admin' | 'hospital'>('hospital')
  const [hcode, setHcode] = useState('')
  const [name, setName] = useState('')
  const [created, setCreated] = useState<APIKeyCreateResponse | null>(null)
  const [copied, setCopied] = useState(false)

  const hospitals = useQuery({ queryKey: ['hospitals'], queryFn: hospitalsApi.list })

  const create = useMutation({
    mutationFn: apiKeysApi.create,
    onSuccess: data => {
      setCreated(data)
      qc.invalidateQueries({ queryKey: ['api-keys'] })
    },
  })

  const hcodes = hospitals.data?.hospitals ?? []

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const body: { role: 'admin' | 'hospital'; hcode?: string; name: string } = { role, name }
    if (role === 'hospital') body.hcode = hcode
    create.mutate(body)
  }

  const handleCopy = async () => {
    if (!created) return
    try {
      await navigator.clipboard.writeText(created.raw_key)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // ignore — clipboard may be unavailable in some contexts
    }
  }

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div className="bg-white rounded-xl w-[520px] max-w-[92vw] shadow-xl">
        <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between">
          <h2 className="text-sm font-medium text-gray-900">
            {created ? 'สร้าง key สำเร็จ' : 'สร้าง API key ใหม่'}
          </h2>
          <button type="button" onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <X className="w-4 h-4" />
          </button>
        </div>

        {!created ? (
          <form onSubmit={handleSubmit}>
            <div className="px-5 py-4 space-y-3">
              <Field label="Role">
                <select
                  required
                  value={role}
                  onChange={e => setRole(e.target.value as 'admin' | 'hospital')}
                  className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm"
                >
                  <option value="hospital">hospital</option>
                  <option value="admin">admin</option>
                </select>
              </Field>
              {role === 'hospital' && (
                <Field label="HCODE">
                  <select
                    required
                    value={hcode}
                    onChange={e => setHcode(e.target.value)}
                    className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono"
                  >
                    <option value="">— เลือกโรงพยาบาล —</option>
                    {hcodes.map(h => (
                      <option key={h.hcode} value={h.hcode}>
                        {h.hcode} — {h.name_th}
                      </option>
                    ))}
                  </select>
                </Field>
              )}
              <Field label="Name">
                <input
                  type="text"
                  required
                  value={name}
                  onChange={e => setName(e.target.value)}
                  placeholder="เช่น รพ.ก. — HIS integration"
                  className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm"
                />
              </Field>
              {create.isError && <ErrorBlock error={create.error} />}
            </div>
            <div className="px-5 py-3 border-t border-gray-100 flex items-center justify-end gap-2">
              <button
                type="button"
                onClick={onClose}
                className="text-sm px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700"
              >
                ยกเลิก
              </button>
              <button
                type="submit"
                disabled={create.isPending}
                className="text-sm px-4 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50"
              >
                {create.isPending ? 'กำลังสร้าง...' : 'สร้าง'}
              </button>
            </div>
          </form>
        ) : (
          <div>
            <div className="px-5 py-4 space-y-3">
              <div className="bg-amber-50 border border-amber-200 rounded-lg p-3 flex items-start gap-2">
                <AlertTriangle className="w-4 h-4 text-amber-700 flex-shrink-0 mt-0.5" />
                <div className="text-xs text-amber-800 font-medium">
                  คัดลอก key ไว้ให้เรียบร้อย — จะไม่แสดงอีก
                </div>
              </div>
              <div className="flex items-center gap-2">
                <code className="flex-1 bg-gray-50 border border-gray-200 rounded-lg px-3 py-2 text-xs font-mono text-gray-900 break-all">
                  {created.raw_key}
                </code>
                <button
                  type="button"
                  onClick={handleCopy}
                  className="text-xs px-3 py-2 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700 inline-flex items-center gap-1.5 flex-shrink-0"
                >
                  {copied ? <Check className="w-3.5 h-3.5 text-green-600" /> : <Copy className="w-3.5 h-3.5" />}
                  {copied ? 'คัดลอกแล้ว' : 'คัดลอก'}
                </button>
              </div>
              <KeyMetaRow created={created} />
            </div>
            <div className="px-5 py-3 border-t border-gray-100 flex items-center justify-end">
              <button
                type="button"
                onClick={onClose}
                className="text-sm px-4 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800"
              >
                ปิด
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function KeyMetaRow({ created }: { created: APIKey }) {
  return (
    <dl className="grid grid-cols-3 gap-2 text-xs text-gray-600 bg-gray-50 border border-gray-100 rounded-lg p-3">
      <div>
        <dt className="text-gray-400">Name</dt>
        <dd className="text-gray-900 mt-0.5">{created.name}</dd>
      </div>
      <div>
        <dt className="text-gray-400">Role</dt>
        <dd className="mt-0.5"><RoleBadge role={created.role} /></dd>
      </div>
      <div>
        <dt className="text-gray-400">HCODE</dt>
        <dd className="font-mono text-gray-900 mt-0.5">{created.hcode || '—'}</dd>
      </div>
    </dl>
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

function formatDT(iso: string): string {
  try {
    return new Date(iso).toLocaleString('th-TH', { hour12: false, dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return iso
  }
}
