package sharefile

import (
	"fmt"
	"strconv"
)

// Canonical filenames used by the IPD share-file export.
const (
	FilePAT = "PAT.csv"
	FileIPD = "IPD.csv"
	FileIDX = "IDX.csv"
	FileIOP = "IOP.csv"
	FileDRU = "DRU.csv"
	FileCHT = "CHT.csv"
	FileCHA = "CHA.csv"
	FileAER = "AER.csv"
	FileIRF = "IRF.csv"
	FileLVD = "LVD.csv"
)

// RequiredFiles lists CSVs every export must include (patient + admission).
var RequiredFiles = []string{FilePAT, FileIPD}

// Row structs per CSV file ตาม NexClaim_HIS_Integration_Spec.

type PATRow struct {
	PID       string
	Prefix    string
	FirstName string
	LastName  string
	DOB       string // YYYYMMDD ค.ศ.
	Sex       string
	Marriage  string
	Nation    string
	Changwat  string
	Amphur    string
	Address   string
}

type IPDRow struct {
	AN          string
	PID         string
	HN          string
	INSCL       string
	PermitNo    string
	AgencyCode  string
	DateAdm     string
	TimeAdm     string
	DateDsc     string
	TimeDsc     string
	WardAdmit   string
	WardDsc     string
	RoomType    string
	LOS         int
	Dischs      string
	Discht      string
	DRGCode     string
	AdjRW       float64
	DoctorCode  string
	UUC         string
}

type IDXRow struct {
	AN         string
	ICD10      string
	DxType     string
	DoctorCode string
}

type IOPRow struct {
	AN         string
	ICD9CM     string
	OpDate     string
	OpTime     string
	DoctorCode string
	Charge     float64
}

type DRURow struct {
	AN            string
	HISItemID     string
	HISItemName   string
	TMTTP         string
	TMT24         string
	Quantity      float64
	Unit          string
	UnitPrice     float64
	TotalPrice    float64
	DrugDateStart string
	DrugDateEnd   string
	Usage         string
	DoctorCode    string
}

type CHTRow struct {
	AN          string
	TotalCharge float64
	TotalClaim  float64
	TotalCopay  float64
}

type CHARow struct {
	AN       string
	ChrgItem string
	Amount   float64
}

type AERRow struct {
	AN     string
	AEDate string
	AETime string
	AEType string
	Cause  string
	Place  string
}

type IRFRow struct {
	AN         string
	ReferFrom  string
	ReferTo    string
	ReferDate  string
	ReferCause string
}

type LVDRow struct {
	AN         string
	LeaveDate  string
	LeaveDays  int
}

// Bundle = ทุก row ที่ parse ได้จาก share folder (ก่อน assemble เป็น IPDAdmit).
type Bundle struct {
	Manifest *Manifest
	PAT      []PATRow
	IPD      []IPDRow
	IDX      []IDXRow
	IOP      []IOPRow
	DRU      []DRURow
	CHT      []CHTRow
	CHA      []CHARow
	AER      []AERRow
	IRF      []IRFRow
	LVD      []LVDRow
}

// Parse อ่าน CSV ทุกไฟล์ใน manifest จาก folder dir. Missing optional files = skipped.
// Required files (PAT.csv, IPD.csv) ถ้าหายจะ error จาก Manifest.Validate ก่อนหน้านี้.
func Parse(dir string, m *Manifest) (*Bundle, error) {
	b := &Bundle{Manifest: m}
	listed := make(map[string]bool, len(m.Files))
	for _, f := range m.Files {
		listed[f] = true
	}

	var err error
	if b.PAT, err = parsePAT(dir); err != nil {
		return nil, err
	}
	if b.IPD, err = parseIPD(dir); err != nil {
		return nil, err
	}
	if listed[FileIDX] {
		if b.IDX, err = parseIDX(dir); err != nil {
			return nil, err
		}
	}
	if listed[FileIOP] {
		if b.IOP, err = parseIOP(dir); err != nil {
			return nil, err
		}
	}
	if listed[FileDRU] {
		if b.DRU, err = parseDRU(dir); err != nil {
			return nil, err
		}
	}
	if listed[FileCHT] {
		if b.CHT, err = parseCHT(dir); err != nil {
			return nil, err
		}
	}
	if listed[FileCHA] {
		if b.CHA, err = parseCHA(dir); err != nil {
			return nil, err
		}
	}
	if listed[FileAER] {
		if b.AER, err = parseAER(dir); err != nil {
			return nil, err
		}
	}
	if listed[FileIRF] {
		if b.IRF, err = parseIRF(dir); err != nil {
			return nil, err
		}
	}
	if listed[FileLVD] {
		if b.LVD, err = parseLVD(dir); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func parsePAT(dir string) ([]PATRow, error) {
	rows, err := ReadPipeCSV(dir, "PAT.csv")
	if err != nil {
		return nil, err
	}
	out := make([]PATRow, 0, len(rows))
	for i, r := range rows {
		p := PATRow{
			PID: r["pid"], Prefix: r["prefix"],
			FirstName: r["first_name"], LastName: r["last_name"],
			DOB: r["dob"], Sex: r["sex"], Marriage: r["marriage"],
			Nation: r["nation"], Changwat: r["changwat"], Amphur: r["amphur"],
			Address: r["address"],
		}
		if p.PID == "" {
			return nil, fmt.Errorf("PAT.csv row %d: pid required", i+1)
		}
		out = append(out, p)
	}
	return out, nil
}

func parseIPD(dir string) ([]IPDRow, error) {
	rows, err := ReadPipeCSV(dir, "IPD.csv")
	if err != nil {
		return nil, err
	}
	out := make([]IPDRow, 0, len(rows))
	for i, r := range rows {
		p := IPDRow{
			AN: r["an"], PID: r["pid"], HN: r["hn"],
			INSCL: r["inscl"], PermitNo: r["permit_no"], AgencyCode: r["agency_code"],
			DateAdm: r["date_adm"], TimeAdm: r["time_adm"],
			DateDsc: r["date_dsc"], TimeDsc: r["time_dsc"],
			WardAdmit: r["ward_admit"], WardDsc: r["ward_dsc"],
			RoomType: r["room_type"],
			LOS:      mustInt(r["los"]),
			Dischs:   r["dischs"], Discht: r["discht"],
			DRGCode:    r["drg_code"],
			AdjRW:      mustFloat(r["adj_rw"]),
			DoctorCode: r["doctor_code"],
			UUC:        r["uuc"],
		}
		if p.AN == "" || p.PID == "" {
			return nil, fmt.Errorf("IPD.csv row %d: an + pid required", i+1)
		}
		out = append(out, p)
	}
	return out, nil
}

func parseIDX(dir string) ([]IDXRow, error) {
	rows, err := ReadPipeCSV(dir, "IDX.csv")
	if err != nil {
		return nil, err
	}
	out := make([]IDXRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, IDXRow{
			AN: r["an"], ICD10: r["icd10"],
			DxType: r["dx_type"], DoctorCode: r["doctor_code"],
		})
	}
	return out, nil
}

func parseIOP(dir string) ([]IOPRow, error) {
	rows, err := ReadPipeCSV(dir, "IOP.csv")
	if err != nil {
		return nil, err
	}
	out := make([]IOPRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, IOPRow{
			AN: r["an"], ICD9CM: r["icd9cm"],
			OpDate: r["op_date"], OpTime: r["op_time"],
			DoctorCode: r["doctor_code"],
			Charge:     mustFloat(r["charge"]),
		})
	}
	return out, nil
}

func parseDRU(dir string) ([]DRURow, error) {
	rows, err := ReadPipeCSV(dir, "DRU.csv")
	if err != nil {
		return nil, err
	}
	out := make([]DRURow, 0, len(rows))
	for _, r := range rows {
		out = append(out, DRURow{
			AN: r["an"], HISItemID: r["his_item_id"], HISItemName: r["his_item_name"],
			TMTTP: r["tmt_tp"], TMT24: r["tmt24"],
			Quantity: mustFloat(r["quantity"]),
			Unit:     r["unit"],
			UnitPrice:  mustFloat(r["unit_price"]),
			TotalPrice: mustFloat(r["total_price"]),
			DrugDateStart: r["drug_date_start"], DrugDateEnd: r["drug_date_end"],
			Usage: r["usage"], DoctorCode: r["doctor_code"],
		})
	}
	return out, nil
}

func parseCHT(dir string) ([]CHTRow, error) {
	rows, err := ReadPipeCSV(dir, "CHT.csv")
	if err != nil {
		return nil, err
	}
	out := make([]CHTRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, CHTRow{
			AN:          r["an"],
			TotalCharge: mustFloat(r["total_charge"]),
			TotalClaim:  mustFloat(r["total_claim"]),
			TotalCopay:  mustFloat(r["total_copay"]),
		})
	}
	return out, nil
}

func parseCHA(dir string) ([]CHARow, error) {
	rows, err := ReadPipeCSV(dir, "CHA.csv")
	if err != nil {
		return nil, err
	}
	out := make([]CHARow, 0, len(rows))
	for _, r := range rows {
		out = append(out, CHARow{
			AN: r["an"], ChrgItem: r["chrgitem"],
			Amount: mustFloat(r["amount"]),
		})
	}
	return out, nil
}

func parseAER(dir string) ([]AERRow, error) {
	rows, err := ReadPipeCSV(dir, "AER.csv")
	if err != nil {
		return nil, err
	}
	out := make([]AERRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, AERRow{
			AN: r["an"], AEDate: r["ae_date"], AETime: r["ae_time"],
			AEType: r["ae_type"], Cause: r["cause"], Place: r["place"],
		})
	}
	return out, nil
}

func parseIRF(dir string) ([]IRFRow, error) {
	rows, err := ReadPipeCSV(dir, "IRF.csv")
	if err != nil {
		return nil, err
	}
	out := make([]IRFRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, IRFRow{
			AN: r["an"], ReferFrom: r["refer_from"], ReferTo: r["refer_to"],
			ReferDate: r["refer_date"], ReferCause: r["refer_cause"],
		})
	}
	return out, nil
}

func parseLVD(dir string) ([]LVDRow, error) {
	rows, err := ReadPipeCSV(dir, "LVD.csv")
	if err != nil {
		return nil, err
	}
	out := make([]LVDRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, LVDRow{
			AN: r["an"], LeaveDate: r["leave_date"],
			LeaveDays: mustInt(r["leave_days"]),
		})
	}
	return out, nil
}

// mustInt returns 0 on empty or parse error (spec: empty = missing).
func mustInt(s string) int {
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

func mustFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
