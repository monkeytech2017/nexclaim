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
