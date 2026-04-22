'use client'
import { useMemo, useState } from 'react'
import { Upload, X, AlertCircle, CheckCircle2 } from 'lucide-react'
import { parseCSV } from '@/lib/csv'

export interface BulkImportResult {
  total:    number
  imported: number
  errors?:  Array<{ row: number; key?: string; reason: string }>
}

type Props<T> = {
  title:     string
  /** Required column names in the CSV header. Rows missing any are flagged. */
  required:  string[]
  /** Convert a parsed row (Record<string,string>) → API item of type T. */
  toItem:    (row: Record<string, string>) => T
  /** POST to the bulk endpoint and return the summary. */
  onSubmit:  (items: T[]) => Promise<BulkImportResult>
  onClose:   () => void
  onDone:    () => void
  /** Link text for the downloadable sample (optional). */
  sampleHint?: string
}

export function BulkImportModal<T>({
  title, required, toItem, onSubmit, onClose, onDone, sampleHint,
}: Props<T>) {
  const [raw, setRaw] = useState('')
  const [result, setResult] = useState<BulkImportResult | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const parsed = useMemo(() => raw.trim() ? parseCSV(raw) : null, [raw])
  const missingHeaders = useMemo(() => {
    if (!parsed) return []
    return required.filter(r => !parsed.headers.includes(r))
  }, [parsed, required])

  async function handleFile(f: File) {
    const txt = await f.text()
    setRaw(txt)
    setResult(null)
    setError(null)
  }

  async function submit() {
    if (!parsed || missingHeaders.length > 0) return
    setSubmitting(true)
    setError(null)
    try {
      const items = parsed.rows.map(toItem)
      const res = await onSubmit(items)
      setResult(res)
      if (res.imported > 0) onDone()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-xl w-[720px] max-w-full max-h-[90vh] flex flex-col shadow-xl">
        <div className="px-5 py-3.5 border-b border-gray-100 flex items-center justify-between flex-shrink-0">
          <h2 className="text-sm font-medium text-gray-900">{title}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="px-5 py-4 overflow-auto flex-1 space-y-3">
          <div className="text-xs text-gray-600 bg-gray-50 border border-gray-100 rounded-lg p-3">
            <p>คอลัมน์ที่ต้องมี: {required.map(r => (
              <code key={r} className="bg-white border border-gray-200 rounded px-1.5 py-0.5 mx-0.5 font-mono text-[11px]">{r}</code>
            ))}</p>
            <p className="mt-1 text-gray-500">
              ไฟล์ CSV/TSV (คั่นด้วย comma, pipe, tab หรือ semicolon — parser จะ detect เอง).
              บรรทัดแรก = header. {sampleHint ?? ''}
            </p>
          </div>

          <div className="flex items-center gap-3">
            <label className="inline-flex items-center gap-2 text-xs px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 cursor-pointer">
              <Upload className="w-3.5 h-3.5" /> เลือกไฟล์ CSV
              <input type="file" accept=".csv,.tsv,.txt,text/csv"
                className="hidden"
                onChange={e => {
                  const f = e.target.files?.[0]
                  if (f) handleFile(f)
                }} />
            </label>
            {parsed && <span className="text-xs text-gray-500">
              {parsed.rows.length} แถว · delimiter <code className="bg-gray-100 px-1 rounded">{parsed.delimiter === '\t' ? 'TAB' : parsed.delimiter}</code>
            </span>}
          </div>

          <details className="text-xs">
            <summary className="cursor-pointer text-gray-600 hover:text-gray-800">หรือวางข้อความ</summary>
            <textarea
              value={raw}
              onChange={e => { setRaw(e.target.value); setResult(null); setError(null) }}
              rows={8}
              className="w-full border border-gray-200 rounded-lg p-2 mt-2 text-[11px] font-mono"
              placeholder="hcode|his_drug_code|tmt_code|..."
            />
          </details>

          {parsed && missingHeaders.length > 0 && (
            <div className="bg-amber-50 border border-amber-100 rounded-lg p-3 text-xs text-amber-900 flex gap-2">
              <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
              <div>
                คอลัมน์ที่ขาด: {missingHeaders.map(h => <code key={h} className="mx-0.5 bg-white px-1.5 py-0.5 rounded border border-amber-200 font-mono">{h}</code>)}
              </div>
            </div>
          )}

          {parsed && missingHeaders.length === 0 && parsed.rows.length > 0 && (
            <div className="bg-white border border-gray-100 rounded-lg overflow-hidden">
              <div className="px-3 py-2 bg-gray-50 text-xs text-gray-500 border-b border-gray-100">
                preview 5 แถวแรก
              </div>
              <table className="w-full text-[11px]">
                <thead>
                  <tr className="bg-white text-gray-500 border-b border-gray-100">
                    {parsed.headers.map(h => (
                      <th key={h} className="px-2 py-1.5 text-left font-medium">
                        {h}{required.includes(h) && <span className="text-red-500">*</span>}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50 font-mono">
                  {parsed.rows.slice(0, 5).map((r, i) => (
                    <tr key={i}>
                      {parsed.headers.map(h => (
                        <td key={h} className="px-2 py-1 text-gray-700">{r[h] || <span className="text-gray-300">—</span>}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {error && (
            <div className="bg-red-50 border border-red-100 rounded-lg p-3 text-xs text-red-800">{error}</div>
          )}

          {result && (
            <div className={`border rounded-lg p-3 text-xs flex gap-2 ${result.errors?.length ? 'bg-amber-50 border-amber-100 text-amber-900' : 'bg-green-50 border-green-100 text-green-900'}`}>
              {result.errors?.length
                ? <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
                : <CheckCircle2 className="w-4 h-4 flex-shrink-0 mt-0.5" />}
              <div className="flex-1 min-w-0">
                <div>นำเข้า <b>{result.imported}</b> / <b>{result.total}</b></div>
                {result.errors && result.errors.length > 0 && (
                  <div className="mt-1.5">
                    <div className="font-medium mb-0.5">ข้อผิดพลาด {result.errors.length} แถว:</div>
                    <ul className="list-disc pl-5 space-y-0.5 max-h-32 overflow-auto">
                      {result.errors.slice(0, 20).map((e, i) => (
                        <li key={i}>
                          row {e.row}
                          {e.key && <code className="mx-1 bg-white/50 px-1 rounded">{e.key}</code>}
                          — {e.reason}
                        </li>
                      ))}
                      {result.errors.length > 20 && <li>… +{result.errors.length - 20} more</li>}
                    </ul>
                  </div>
                )}
              </div>
            </div>
          )}
        </div>

        <div className="px-5 py-3 border-t border-gray-100 flex items-center justify-end gap-2 flex-shrink-0">
          <button onClick={onClose}
            className="text-sm px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700">
            ปิด
          </button>
          <button
            onClick={submit}
            disabled={!parsed || missingHeaders.length > 0 || submitting || parsed.rows.length === 0}
            className="text-sm px-4 py-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-800 disabled:opacity-50"
          >
            {submitting ? 'กำลังนำเข้า...' : `นำเข้า ${parsed?.rows.length ?? 0} แถว`}
          </button>
        </div>
      </div>
    </div>
  )
}
