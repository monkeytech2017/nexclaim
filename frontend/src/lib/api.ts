// API client สำหรับ NexClaim Go backend
// Base URL ชี้ไปที่ /api/backend/* ซึ่ง next.config.js proxy ไป Go :8080

const BASE = '/api/backend'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const err = await res.text()
    throw new Error(`API ${path} → ${res.status}: ${err}`)
  }
  return res.json()
}

// ─── Claim Batch ──────────────────────────────────────────────

export interface ClaimBatch {
  batch_id:      string
  hcode:         string
  period:        string
  inscl:         string
  format:        string
  status:        string
  total_records: number
  valid_records: number
  error_records: number
  fdh_txn_id?:  string
  deadline_at?:  string
  created_at:    string
  sent_at?:      string
}

export const claimApi = {
  list: (params?: { period?: string; inscl?: string; status?: string }) =>
    request<ClaimBatch[]>(`/api/claim/batch?${new URLSearchParams(params as Record<string,string>)}`),

  submit: (inscl: string, period: string, dryRun = false) =>
    request<{ batch_id: string; txn_id: string }>('/api/claim/submit', {
      method: 'POST',
      body: JSON.stringify({ inscl, period, dry_run: dryRun }),
    }),

  status: (txnId: string) =>
    request<{ status: string; message: string }>(`/api/claim/status/${txnId}`),
}

// ─── C-Code ───────────────────────────────────────────────────

export interface CCodeItem {
  id:         string
  batch_id:   string
  hn:         string
  an_or_seq:  string
  c_code:     string
  c_desc:     string
  field_name: string
  resolved:   boolean
  received_at: string
}

export const ccodeApi = {
  list: (batchId?: string, resolved?: boolean) =>
    request<CCodeItem[]>(`/api/ccode?${new URLSearchParams({
      ...(batchId   ? { batch_id: batchId }        : {}),
      ...(resolved !== undefined ? { resolved: String(resolved) } : {}),
    })}`),

  resolve: (id: string) =>
    request<void>(`/api/ccode/${id}/resolve`, { method: 'PATCH' }),
}

// ─── Master Data ──────────────────────────────────────────────

export const masterApi = {
  inscl: () => request<{ inscl: string; name_th: string; format_ipd: string; format_opd: string }[]>('/api/master/inscl'),
  chrgitem: () => request<{ code: string; name_th: string }[]>('/api/master/chrgitem'),
}
