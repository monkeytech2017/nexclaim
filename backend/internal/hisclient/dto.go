// Package hisclient รวม DTO และ HTTP client สำหรับเรียก HIS API
// ตาม spec NexClaim_HIS_Integration_Spec.xlsx
//
// ช่องทาง: OPD 2-Way — NexClaim เป็น client ของ HIS endpoints:
//   GET /api/nexclaim/opd/visit/{vn}
//   GET /api/nexclaim/opd/visits?vn=VN1,VN2
//   GET /api/nexclaim/opd/visits/period/{YYYYMM}
//   GET /api/nexclaim/drug/list
//   GET /api/nexclaim/health
package hisclient

// ── POST payload: HIS → NexClaim /api/v1/his/opd/visits ─────────

// VisitSummary = 1 row ใน `visits` array ที่ HIS push มา (summary-only).
// รายละเอียดจริง NexClaim จะดึงเพิ่มด้วย GET /api/nexclaim/opd/visit/{vn}.
type VisitSummary struct {
	VN          string  `json:"vn"`
	HN          string  `json:"hn"`
	PID         string  `json:"pid"`
	PatientName string  `json:"patient_name"`
	VisitDate   string  `json:"visit_date"` // YYYYMMDD ค.ศ.
	VisitTime   string  `json:"visit_time,omitempty"`
	INSCL       string  `json:"inscl"`
	INSCLName   string  `json:"inscl_name,omitempty"`
	ClinicCode  string  `json:"clinic_code,omitempty"`
	ClinicName  string  `json:"clinic_name,omitempty"`
	DoctorCode  string  `json:"doctor_code,omitempty"`
	TotalCharge float64 `json:"total_charge,omitempty"`
}

// VisitListRequest = body ของ POST /api/v1/his/opd/visits.
type VisitListRequest struct {
	HospitalCode string         `json:"hospital_code"`
	Period       string         `json:"period"`
	ExportedBy   string         `json:"exported_by,omitempty"`
	Visits       []VisitSummary `json:"visits"`
}

// VisitListResponse ส่งกลับทันทีหลังรับ push list.
type VisitListResponse struct {
	Status        string `json:"status"`             // "OK" | "ERROR"
	BatchID       string `json:"batch_id"`
	ReceivedCount int    `json:"received_count"`
	Message       string `json:"message,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"` // RFC3339
}

// VisitListError = error-response body จาก POST visits (400).
type VisitListError struct {
	Status  string             `json:"status"` // "ERROR"
	Error   string             `json:"error"`  // "VALIDATION_ERROR" | ...
	Message string             `json:"message"`
	Errors  []VisitErrorDetail `json:"errors,omitempty"`
}

type VisitErrorDetail struct {
	VN      string `json:"vn,omitempty"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ── GET /api/nexclaim/opd/visit/{vn} response ───────────────────

type VisitDetail struct {
	Patient   VisitPatient    `json:"patient"`
	Visit     VisitInfo       `json:"visit"`
	Insurance VisitInsurance  `json:"insurance"`
	Diagnosis []VisitDiagnose `json:"diagnosis"`
	Procedure []VisitProc     `json:"procedure,omitempty"`
	Drug      []VisitDrug     `json:"drug,omitempty"`
	Charge    []VisitCharge   `json:"charge"`
	Referral  *VisitReferral  `json:"referral,omitempty"`
	Accident  *VisitAccident  `json:"accident,omitempty"`
}

type VisitPatient struct {
	PID       string `json:"pid"`
	Prefix    string `json:"prefix,omitempty"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	DOB       string `json:"dob"` // YYYYMMDD
	Sex       string `json:"sex"`
	Marriage  string `json:"marriage,omitempty"`
	Nation    string `json:"nation,omitempty"`
	Changwat  string `json:"changwat,omitempty"`
	Amphur    string `json:"amphur,omitempty"`
	Address   string `json:"address,omitempty"`
}

type VisitInfo struct {
	VN         string `json:"vn"`
	SEQ        string `json:"seq"`
	Date       string `json:"date"`
	Time       string `json:"time,omitempty"`
	ClinicCode string `json:"clinic_code,omitempty"`
	UUC        string `json:"uuc"`
}

type VisitInsurance struct {
	INSCL      string `json:"inscl"`
	PermitNo   string `json:"permit_no,omitempty"`
	AgencyCode string `json:"agency_code,omitempty"`
}

type VisitDiagnose struct {
	ICD10      string `json:"icd10"`
	DxType     string `json:"dx_type"`
	DoctorCode string `json:"doctor_code,omitempty"`
}

type VisitProc struct {
	ICD9CM     string  `json:"icd9cm"`
	Date       string  `json:"date,omitempty"`
	DoctorCode string  `json:"doctor_code,omitempty"`
	Charge     float64 `json:"charge,omitempty"`
}

type VisitDrug struct {
	HISItemID   string  `json:"his_item_id"`
	HISItemName string  `json:"his_item_name"`
	TMTTP       string  `json:"tmt_tp,omitempty"`
	TMT24       string  `json:"tmt24,omitempty"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	TotalPrice  float64 `json:"total_price"`
	Usage       string  `json:"usage,omitempty"`
	DoctorCode  string  `json:"doctor_code,omitempty"`
}

type VisitCharge struct {
	ChrgItem string  `json:"chrgitem"`
	Amount   float64 `json:"amount"`
	Detail   string  `json:"detail,omitempty"`
}

type VisitReferral struct {
	ReferFrom  string `json:"refer_from,omitempty"`
	ReferTo    string `json:"refer_to,omitempty"`
	ReferDate  string `json:"refer_date,omitempty"`
	ReferCause string `json:"refer_cause,omitempty"`
}

type VisitAccident struct {
	AEDate string `json:"ae_date,omitempty"`
	AETime string `json:"ae_time,omitempty"`
	AEType string `json:"ae_type,omitempty"`
	Cause  string `json:"cause,omitempty"`
	Place  string `json:"place,omitempty"`
}

// ── Batch GET /visits?vn=VN1,VN2 response ───────────────────────

type BatchVisitsResponse struct {
	Status string        `json:"status"` // "OK" | "PARTIAL"
	Count  int           `json:"count"`
	Visits []VisitDetail `json:"visits"`
	Errors []BatchError  `json:"errors,omitempty"`
}

type BatchError struct {
	VN      string `json:"vn"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ── GET /api/nexclaim/drug/list response ────────────────────────

type DrugListResponse struct {
	Status     string     `json:"status"`
	Page       int        `json:"page"`
	Limit      int        `json:"limit"`
	Total      int        `json:"total"`
	TotalPages int        `json:"total_pages"`
	Data       []DrugItem `json:"data"`
}

type DrugItem struct {
	HISItemID   string  `json:"his_item_id"`
	HISItemCode string  `json:"his_item_code,omitempty"`
	HISItemName string  `json:"his_item_name"`
	TMTTP       string  `json:"tmt_tp,omitempty"`
	GenericName string  `json:"generic_name,omitempty"`
	Strength    string  `json:"strength,omitempty"`
	DosageForm  string  `json:"dosage_form,omitempty"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	Active      bool    `json:"active"`
}

// ── GET /api/nexclaim/health ────────────────────────────────────

type HealthResponse struct {
	Status       string             `json:"status"`
	Version      string             `json:"version,omitempty"`
	HospitalCode string             `json:"hospital_code,omitempty"`
	HospitalName string             `json:"hospital_name,omitempty"`
	HISVersion   string             `json:"his_version,omitempty"`
	Database     string             `json:"database,omitempty"`
	Timestamp    string             `json:"timestamp,omitempty"`
	Capabilities *HealthCapabilities `json:"capabilities,omitempty"`
}

type HealthCapabilities struct {
	SingleVisit  bool `json:"single_visit"`
	BatchVisits  bool `json:"batch_visits"`
	PeriodList   bool `json:"period_list"`
	DrugList     bool `json:"drug_list"`
	MaxBatchSize int  `json:"max_batch_size"`
}
