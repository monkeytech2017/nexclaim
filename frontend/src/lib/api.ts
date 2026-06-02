// API client — proxied via next.config.js to Go backend at BACKEND_URL.

const BASE = '/api/backend'

// ── Typed errors ─────────────────────────────────────────────────────────────
// AuthError → missing/invalid API key (HTTP 401).
// ScopeError → key valid but lacks scope for this resource (HTTP 403).
// Both extend Error so existing `error instanceof Error` callers keep working.

export class AuthError extends Error {
  constructor(message = 'ไม่ได้รับอนุญาต (401) — ตรวจสอบ API key') {
    super(message)
    this.name = 'AuthError'
  }
}

export class ScopeError extends Error {
  constructor(message = 'ไม่มีสิทธิ์เข้าถึงข้อมูลนี้ (403)') {
    super(message)
    this.name = 'ScopeError'
  }
}

// ── API key storage (browser localStorage) ──
// Primary: key stored via login page. Fallback: NEXT_PUBLIC_API_KEY env (DEV only).

const API_KEY_STORAGE = 'nexclaim.apiKey'

export const storedApiKey = (): string | null => {
  if (typeof window === 'undefined') return null
  try {
    const v = localStorage.getItem(API_KEY_STORAGE)
    return v && v.length > 0 ? v : null
  } catch {
    return null
  }
}

export const setApiKey = (key: string): void => {
  if (typeof window === 'undefined') return
  localStorage.setItem(API_KEY_STORAGE, key)
}

export const clearApiKey = (): void => {
  if (typeof window === 'undefined') return
  try {
    localStorage.removeItem(API_KEY_STORAGE)
  } catch {
    // ignore — private-mode browsers may throw
  }
}

function authHeader(): Record<string, string> {
  const stored = storedApiKey()
  if (stored) return { Authorization: `Bearer ${stored}` }
  const env = process.env.NEXT_PUBLIC_API_KEY
  if (env) return { Authorization: `Bearer ${env}` }
  return {}
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...authHeader(),
      ...(options?.headers ?? {}),
    },
  })
  if (!res.ok) {
    let msg: string
    try {
      const body = await res.json()
      msg = body.error || body.message || JSON.stringify(body)
    } catch {
      msg = await res.text()
    }
    if (res.status === 401) throw new AuthError(msg || undefined)
    if (res.status === 403) throw new ScopeError(msg || undefined)
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

// ── Auth / whoami ──

export interface Identity {
  id: string
  role: 'admin' | 'hospital'
  hcode?: string
  name: string
}
export const authApi = {
  whoami: () => request<Identity>('/api/v1/auth/whoami'),
}

// ── API keys (admin-only management) ──

export interface APIKey {
  id:            string
  name:          string
  role:          'admin' | 'hospital'
  hcode?:        string
  is_active:     boolean
  created_at:    string
  last_used_at?: string
  // expires_at: nil/absent means "never expires" (back-compat default).
  // When present, the backend rejects the key after this timestamp.
  expires_at?:   string
}

export interface APIKeyCreateResponse extends APIKey {
  raw_key: string   // populated ONLY on POST response — shown to user once
}

export const apiKeysApi = {
  list:       () => request<{ items: APIKey[] }>('/api/v1/auth/keys'),
  // ttl_days: optional integer. 0/absent = no expiry; positive = now + N days.
  // Negative → backend returns 400.
  create:     (body: { role: 'admin' | 'hospital'; hcode?: string; name: string; ttl_days?: number }) =>
                request<APIKeyCreateResponse>('/api/v1/auth/keys', { method: 'POST', body: JSON.stringify(body) }),
  setActive:  (id: string, is_active: boolean) =>
                request<APIKey>(`/api/v1/auth/keys/${encodeURIComponent(id)}`, {
                  method: 'PATCH', body: JSON.stringify({ is_active }),
                }),
}

// ── Audit log (admin-only, append-only) ──

export interface AuditEntry {
  id:           string
  actor_id?:    string
  actor_role?:  string
  actor_name?:  string
  action:       string
  target_kind?: string
  target_id?:   string
  hcode?:       string
  payload?:     Record<string, unknown>
  created_at:   string
}

export const auditApi = {
  list: (filter: {
    action?:       string
    actor_id?:     string
    hcode?:        string
    target_kind?:  string
    target_id?:    string
    from?:         string
    to?:           string
    limit?:        number
  }) => {
    const qs = new URLSearchParams()
    if (filter.action)      qs.set('action', filter.action)
    if (filter.actor_id)    qs.set('actor_id', filter.actor_id)
    if (filter.hcode)       qs.set('hcode', filter.hcode)
    if (filter.target_kind) qs.set('target_kind', filter.target_kind)
    if (filter.target_id)   qs.set('target_id', filter.target_id)
    if (filter.from)        qs.set('from', filter.from)
    if (filter.to)          qs.set('to', filter.to)
    if (filter.limit)       qs.set('limit', String(filter.limit))
    const suffix = qs.toString() ? `?${qs}` : ''
    return request<{ items: AuditEntry[] }>(`/api/v1/audit-log${suffix}`)
  },
}

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

// ── Master data: Hospitals ──

export interface Hospital {
  hcode:       string
  name_th:     string
  changwat?:   string
  amphur?:     string
  his_db_key?: string
  is_active:   boolean
  created_at?: string
}

export const hospitalsApi = {
  list:   () => request<{ hospitals: Hospital[] }>('/api/v1/master/hospitals'),
  get:    (hcode: string) => request<Hospital>(`/api/v1/master/hospitals/${encodeURIComponent(hcode)}`),
  create: (h: Hospital) =>
    request<Hospital>('/api/v1/master/hospitals', {
      method: 'POST',
      body: JSON.stringify(h),
    }),
  update: (hcode: string, h: Partial<Hospital>) =>
    request<Hospital>(`/api/v1/master/hospitals/${encodeURIComponent(hcode)}`, {
      method: 'PATCH',
      body: JSON.stringify({ ...h, hcode }),
    }),
  delete: (hcode: string) =>
    request<void>(`/api/v1/master/hospitals/${encodeURIComponent(hcode)}`, { method: 'DELETE' }),
}

// ── Master data: Doctors ──

export interface Doctor {
  doctor_id:  string
  hcode:      string
  license_no: string
  name_th?:   string
  specialty?: string
  is_active:  boolean
  updated_at?: string
}

export const doctorsApi = {
  list:   (hcode?: string) =>
    request<{ doctors: Doctor[] }>(`/api/v1/master/doctors${hcode ? `?hcode=${encodeURIComponent(hcode)}` : ''}`),
  get:    (id: string) => request<Doctor>(`/api/v1/master/doctors/${encodeURIComponent(id)}`),
  create: (d: Doctor) =>
    request<Doctor>('/api/v1/master/doctors', { method: 'POST', body: JSON.stringify(d) }),
  update: (id: string, d: Partial<Doctor>) =>
    request<Doctor>(`/api/v1/master/doctors/${encodeURIComponent(id)}`, {
      method: 'PATCH', body: JSON.stringify({ ...d, doctor_id: id }),
    }),
  delete: (id: string) =>
    request<void>(`/api/v1/master/doctors/${encodeURIComponent(id)}`, { method: 'DELETE' }),
}

// ── Master data: INSCL mapping (per hospital) ──

export interface InsclMap {
  hcode:        string
  his_pttype:   string
  inscl:        string
  agency_code?: string
  note?:        string
}

export const insclMapsApi = {
  list:   (hcode?: string) =>
    request<{ items: InsclMap[] }>(`/api/v1/master/inscl-maps${hcode ? `?hcode=${encodeURIComponent(hcode)}` : ''}`),
  upsert: (m: InsclMap) =>
    request<InsclMap>('/api/v1/master/inscl-maps', { method: 'POST', body: JSON.stringify(m) }),
  delete: (hcode: string, hisPttype: string) =>
    request<void>(`/api/v1/master/inscl-maps/${encodeURIComponent(hcode)}/${encodeURIComponent(hisPttype)}`,
      { method: 'DELETE' }),
}

// ── Master data: HIS drug mapping ──

export interface DrugMap {
  id?:            string
  hcode:          string
  his_drug_code:  string
  tmt_code?:      string
  his_drug_name?: string
  note?:          string
  is_active:      boolean
}

export interface BulkResult {
  total:    number
  imported: number
  errors?:  Array<{ row: number; key?: string; reason: string }>
}

export const drugMapsApi = {
  list:   (hcode?: string) =>
    request<{ items: DrugMap[] }>(`/api/v1/master/drug-maps${hcode ? `?hcode=${encodeURIComponent(hcode)}` : ''}`),
  upsert: (m: DrugMap) =>
    request<DrugMap>('/api/v1/master/drug-maps', { method: 'POST', body: JSON.stringify(m) }),
  delete: (hcode: string, hisDrugCode: string) =>
    request<void>(`/api/v1/master/drug-maps/${encodeURIComponent(hcode)}/${encodeURIComponent(hisDrugCode)}`,
      { method: 'DELETE' }),
  bulk:   (items: DrugMap[]) =>
    request<BulkResult>('/api/v1/master/drug-maps/bulk', {
      method: 'POST', body: JSON.stringify({ items }),
    }),
}

// ── Master data: HIS doctor mapping ──

export interface DoctorMap {
  hcode:            string
  his_doctor_code:  string
  doctor_id?:       string
}

export const doctorMapsApi = {
  list:   (hcode?: string) =>
    request<{ items: DoctorMap[] }>(`/api/v1/master/doctor-maps${hcode ? `?hcode=${encodeURIComponent(hcode)}` : ''}`),
  upsert: (m: DoctorMap) =>
    request<DoctorMap>('/api/v1/master/doctor-maps', { method: 'POST', body: JSON.stringify(m) }),
  delete: (hcode: string, hisDoctorCode: string) =>
    request<void>(`/api/v1/master/doctor-maps/${encodeURIComponent(hcode)}/${encodeURIComponent(hisDoctorCode)}`,
      { method: 'DELETE' }),
  bulk:   (items: DoctorMap[]) =>
    request<BulkResult>('/api/v1/master/doctor-maps/bulk', {
      method: 'POST', body: JSON.stringify({ items }),
    }),
}

// ── Master data: HIS ICD mapping ──

export type IcdType = '10' | '9C'

export interface IcdMap {
  hcode:        string
  his_icd_code: string
  icd_type:     IcdType
  std_code:     string
}

export const icdMapsApi = {
  list: (hcode?: string, type?: IcdType) => {
    const qs = new URLSearchParams()
    if (hcode) qs.set('hcode', hcode)
    if (type)  qs.set('type', type)
    const suffix = qs.toString() ? `?${qs}` : ''
    return request<{ items: IcdMap[] }>(`/api/v1/master/icd-maps${suffix}`)
  },
  upsert: (m: IcdMap) =>
    request<IcdMap>('/api/v1/master/icd-maps', { method: 'POST', body: JSON.stringify(m) }),
  delete: (hcode: string, icdType: IcdType, hisIcdCode: string) =>
    request<void>(
      `/api/v1/master/icd-maps/${encodeURIComponent(hcode)}/${encodeURIComponent(icdType)}/${encodeURIComponent(hisIcdCode)}`,
      { method: 'DELETE' }),
  bulk:   (items: IcdMap[]) =>
    request<BulkResult>('/api/v1/master/icd-maps/bulk', {
      method: 'POST', body: JSON.stringify({ items }),
    }),
}

// ── Master data: HIS field mapping (column → target spec) ──

export interface FieldMap {
  id?:            string
  hcode:          string
  his_table:      string
  his_column:     string
  target_file:    string
  target_field:   string
  transform?:     string
  is_required:    boolean
  default_value?: string
  note?:          string
}

export const fieldMapsApi = {
  list: (filter: { hcode?: string; his_table?: string; target_file?: string }) => {
    const qs = new URLSearchParams()
    if (filter.hcode)       qs.set('hcode', filter.hcode)
    if (filter.his_table)   qs.set('his_table', filter.his_table)
    if (filter.target_file) qs.set('target_file', filter.target_file)
    const suffix = qs.toString() ? `?${qs}` : ''
    return request<{ items: FieldMap[] }>(`/api/v1/master/field-maps${suffix}`)
  },
  upsert: (m: FieldMap) =>
    request<FieldMap>('/api/v1/master/field-maps', { method: 'POST', body: JSON.stringify(m) }),
  delete: (id: string) =>
    request<void>(`/api/v1/master/field-maps/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  bulk:   (items: FieldMap[]) =>
    request<BulkResult>('/api/v1/master/field-maps/bulk', {
      method: 'POST', body: JSON.stringify({ items }),
    }),
}

// ── Submission history (claim_batch + joined c-code counts) ──

export interface ClaimBatch {
  batch_id:      string
  hcode:         string
  period:        string
  inscl:         string
  format:        string
  sender:        string
  // status values: 'pending' | 'sent' | 'error' | 'failed'
  // 'failed' is a terminal state reached after max_attempts retries.
  status:        string
  total_records: number
  valid_records: number
  error_records: number
  fdh_txn_id?:   string
  zip_filename?: string
  zip_md5?:      string
  created_at:    string
  sent_at?:      string
  error_msg?:    string
  c_code_count:  number
  c_code_open:   number
  // attempt_no = count of send attempts so far (1 = initial run, 2+ = retries).
  attempt_no:    number
  // next_retry_at populated while the retry worker still owns the batch.
  next_retry_at?: string
}

export const claimBatchesApi = {
  list: (filter: { hcode?: string; period?: string; inscl?: string; format?: string; status?: string }) => {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(filter)) if (v) qs.set(k, v)
    const suffix = qs.toString() ? `?${qs}` : ''
    return request<{ items: ClaimBatch[] }>(`/api/v1/claim/batches${suffix}`)
  },
  get: (id: string) =>
    request<ClaimBatch>(`/api/v1/claim/batches/${encodeURIComponent(id)}`),
}

// ── C-code (REP feedback) ──

export interface CCode {
  id:           string
  batch_id:     string
  record_id?:   string
  hn?:          string
  an_or_seq?:   string
  c_code:       string
  c_desc?:      string
  field_name?:  string
  field_value?: string
  resolved:     boolean
  resolved_by?: string
  resolved_at?: string
  received_at:  string
}

export interface IngestResult {
  fetched:  number
  inserted: number
  skipped:  number
  errors:   number
}

export const ccodesApi = {
  list: (filter: { batch_id?: string; hcode?: string; period?: string; resolved?: boolean; c_code?: string }) => {
    const qs = new URLSearchParams()
    if (filter.batch_id) qs.set('batch_id', filter.batch_id)
    if (filter.hcode)    qs.set('hcode', filter.hcode)
    if (filter.period)   qs.set('period', filter.period)
    if (filter.c_code)   qs.set('c_code', filter.c_code)
    if (filter.resolved !== undefined) qs.set('resolved', String(filter.resolved))
    const suffix = qs.toString() ? `?${qs}` : ''
    return request<{ items: CCode[] }>(`/api/v1/ccodes${suffix}`)
  },
  resolve: (id: string, resolvedBy: string) =>
    request<void>(`/api/v1/ccodes/${encodeURIComponent(id)}/resolve`, {
      method: 'PATCH', body: JSON.stringify({ resolved_by: resolvedBy }),
    }),
  fetchRep: (hcode: string, period: string) =>
    request<IngestResult>(
      `/api/v1/claim/rep/${encodeURIComponent(hcode)}/${encodeURIComponent(period)}`,
      { method: 'POST' }),
}

// ── Send-log audit trail (every outbound FDH/CHI attempt) ──

export interface SendLog {
  id:             string
  batch_id:       string
  attempt_no:     number
  endpoint:       string
  http_status?:   number
  fdh_txn_id?:    string
  response_body?: string
  duration_ms?:   number
  success?:       boolean
  error_msg?:     string
  sent_at:        string
  hcode:          string
  period:         string
  inscl:          string
  format:         string
}

export const sendLogsApi = {
  list: (filter: { batch_id?: string; hcode?: string; period?: string; success?: 'true' | 'false' | '' }) => {
    const qs = new URLSearchParams()
    if (filter.batch_id) qs.set('batch_id', filter.batch_id)
    if (filter.hcode)    qs.set('hcode', filter.hcode)
    if (filter.period)   qs.set('period', filter.period)
    if (filter.success)  qs.set('success', filter.success)
    const suffix = qs.toString() ? `?${qs}` : ''
    return request<{ items: SendLog[] }>(`/api/v1/send-logs${suffix}`)
  },
}

// ── Dashboard stats ──

export interface DashboardStats {
  summary: {
    batches_total:     number
    records_total:     number
    records_errors:    number
    ccodes_open:       number
    avg_send_ms:       number
    send_success_rate: number    // 0..1
  }
  by_format: { format: string; count: number; records: number }[]
  by_status: { status: string; count: number }[]
  by_day:    { day: string; formats: Record<string, number> }[]
  top_ccodes:{ c_code: string; count: number; desc_sample: string }[]
}

export const dashboardApi = {
  stats: (filter: { hcode?: string; period_from?: string; period_to?: string }) => {
    const qs = new URLSearchParams()
    if (filter.hcode)       qs.set('hcode', filter.hcode)
    if (filter.period_from) qs.set('period_from', filter.period_from)
    if (filter.period_to)   qs.set('period_to', filter.period_to)
    const suffix = qs.toString() ? `?${qs}` : ''
    return request<DashboardStats>(`/api/v1/dashboard/stats${suffix}`)
  },
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
