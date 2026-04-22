// API client — proxied via next.config.js to Go backend at BACKEND_URL.

const BASE = '/api/backend'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    let msg: string
    try {
      const body = await res.json()
      msg = body.error || body.message || JSON.stringify(body)
    } catch {
      msg = await res.text()
    }
    throw new Error(`${res.status}: ${msg}`)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

// ── Health ──

export interface HealthResponse { ok: boolean; service: string }
export const health = () => request<HealthResponse>('/healthz')

// ── Shared types (mirrors backend DTOs) ──

export type INSCL =
  | '011' | 'WEL' | 'LGO' | 'OFC'
  | 'UCS' | 'NON' | 'WP1' | 'WP2'
  | 'SSS' | 'SS4'
  | 'TPBS' | 'WK' | 'MON' | 'PRS'

export type ClaimFormat = '16FILES' | 'CIPN' | 'CSOP' | 'AIPN' | 'SSOP'

export interface ValidationError {
  field:  string
  value?: string
  cCode?: string
  reason: string
}

export interface Submission {
  format:    ClaimFormat
  zipName?:  string
  zipBytes:  number
  xmlBytes:  number
  filesN:    number
  txnId?:    string
  status?:   string
  message?:  string
  error?:    string
}

export interface SubmitResponse {
  inscl:              string
  opdCount:           number
  ipdCount:           number
  validationErrors?:  ValidationError[]
  submissions:        Submission[] | null
}

// ── /api/submit (dev/admin direct pipeline trigger) ──

export interface DirectSubmitRequest {
  inscl:   string
  period:  string
  hcode?:  string
  agency?: string
  dryRun?: boolean
}

export const submitDirect = (body: DirectSubmitRequest) =>
  request<SubmitResponse>('/api/submit', {
    method: 'POST',
    body: JSON.stringify(body),
  })

// ── /api/status/:txnId ──

export interface StatusResponse { txnId: string; status: string; message: string }
export const getStatus = (txnId: string) =>
  request<StatusResponse>(`/api/status/${encodeURIComponent(txnId)}`)

// ── OPD 2-Way: visits + batches ──

export interface VisitSummary {
  vn:            string
  hn:            string
  pid:           string
  patient_name?: string
  visit_date:    string    // YYYYMMDD ค.ศ.
  visit_time?:   string
  inscl:         string
  inscl_name?:   string
  clinic_code?:  string
  clinic_name?:  string
  doctor_code?:  string
  total_charge?: number
}

export interface VisitListRequest {
  hospital_code: string
  period:        string
  exported_by?:  string
  visits:        VisitSummary[]
}

export interface VisitListResponse {
  status:         string
  batch_id:       string
  received_count: number
  message?:       string
  created_at?:    string
}

export type BatchState = 'RECEIVED' | 'FETCHING' | 'COMPLETED' | 'FAILED'

export interface Batch {
  batch_id:      string
  hospital_code: string
  period:        string
  exported_by?:  string
  state:         BatchState
  created_at:    string
  updated_at:    string
  visits:        VisitSummary[]
  last_error?:   string
}

export interface ProcessBatchRun {
  inscl:       string
  vn_count:    number
  outcome?:    SubmitResponse
  error?:      string
}

export interface ProcessBatchResponse {
  batchId: string
  dryRun:  boolean
  runs:    ProcessBatchRun[]
}

export const opdApi = {
  pushVisits: (body: VisitListRequest) =>
    request<VisitListResponse>('/api/v1/his/opd/visits', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  listBatches: () =>
    request<{ batches: Batch[] | null }>('/api/v1/his/opd/batches'),

  getBatch: (batchId: string) =>
    request<Batch>(`/api/v1/his/opd/batches/${encodeURIComponent(batchId)}`),

  processBatch: (batchId: string, dryRun = false) =>
    request<ProcessBatchResponse>(
      `/api/v1/his/opd/batches/${encodeURIComponent(batchId)}/process${dryRun ? '?dry_run=true' : ''}`,
      { method: 'POST' },
    ),
}

// ── IPD Share Folder ──

export interface IPDImportEntry {
  export_id: string
  ready:     boolean
  path:      string
}

export interface IPDImportRun {
  inscl:        string
  admit_count:  number
  outcome?:     SubmitResponse
  error?:       string
}

export interface IPDImportResult {
  export_id:  string
  dryRun:     boolean
  runs:       IPDImportRun[]
  moveError?: string
}

export const ipdApi = {
  listImports: () =>
    request<{ imports: IPDImportEntry[] | null }>('/api/v1/his/ipd/imports'),

  runImport: (exportId: string, dryRun = false) =>
    request<IPDImportResult>(
      `/api/v1/his/ipd/imports/${encodeURIComponent(exportId)}${dryRun ? '?dry_run=true' : ''}`,
      { method: 'POST' },
    ),
}

// ── Labels ──

export const INSCL_LABELS: Record<INSCL, string> = {
  '011':  'ข้าราชการพลเรือน',
  'WEL':  'ลูกจ้างประจำ/บำนาญ',
  'LGO':  'อปท.',
  'OFC':  'หน่วยงานอิสระ',
  'UCS':  'บัตรทอง',
  'NON':  'ไร้สัญชาติ',
  'WP1':  'แรงงานต่างด้าว MOU',
  'WP2':  'แรงงานต่างด้าวขึ้นทะเบียน',
  'SSS':  'ประกันสังคม ม.33/39',
  'SS4':  'ประกันสังคม ม.40',
  'TPBS': 'พ.ร.บ.รถ',
  'WK':   'กองทุนทดแทน',
  'MON':  'พระภิกษุ',
  'PRS':  'ราชทัณฑ์',
}
