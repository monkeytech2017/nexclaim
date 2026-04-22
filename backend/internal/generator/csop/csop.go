// Package csop สร้าง XML CSOP (OPD ข้าราชการ/อปท./หน่วยงานอิสระ) → ส่ง FDH
//
// โครงสร้างเหมือน CIPN แต่ใช้ VISIT แทน ADMIT, SEQ แทน AN, DATEOPD แทน DATEADM
// และไม่มี DRG (OPD ไม่มี DRG grouping).
package csop

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/util"
)

type Input struct {
	HCode    string
	HCodeSub string
	Agency   model.Agency
	Period   string
	SendDate time.Time
	OPD      []model.OPDVisit
}

type Document struct {
	XMLName xml.Name `xml:"CSOP"`
	Header  Header   `xml:"HEADER"`
	Visits  []Visit  `xml:"VISIT"`
}

type Header struct {
	HospMain string `xml:"HMAIN"`
	HospSub  string `xml:"HSUB"`
	Agency   string `xml:"AGENCY"`
	Period   string `xml:"PERIOD"`
	SendDate string `xml:"SENDDATE"`
}

type Visit struct {
	XMLName   xml.Name `xml:"VISIT"`
	HN        string   `xml:"HN"`
	PID       string   `xml:"PID"`
	SEQ       string   `xml:"SEQ"`
	DateOPD   string   `xml:"DATEOPD"`
	TimeOPD   string   `xml:"TIMEOPD,omitempty"`
	Clinic    string   `xml:"CLINIC,omitempty"`
	TypeOut   string   `xml:"TYPEOUT,omitempty"`
	UUC       string   `xml:"UUC"`
	PermitNo  string   `xml:"PERMITNO,omitempty"`
	Diagnoses []Diag   `xml:"DIAG"`
	Procs     []Proc   `xml:"PROC"`
	Charges   []Charge `xml:"CHARGE"`
	Drugs     []Drug   `xml:"DRUG"`
	Total     float64  `xml:"TOTAL"`
	Paid      float64  `xml:"PAID"`
}

type Diag struct {
	XMLName xml.Name `xml:"DIAG"`
	DxType  string   `xml:"DXTYPE,attr"`
	Code    string   `xml:",chardata"`
}

type Proc struct {
	XMLName xml.Name `xml:"PROC"`
	DrOpID  string   `xml:"DROPID,attr,omitempty"`
	Date    string   `xml:"DATE,attr,omitempty"`
	Code    string   `xml:",chardata"`
}

type Charge struct {
	XMLName  xml.Name `xml:"CHARGE"`
	ChrgItem string   `xml:"CHRGITEM,attr"`
	Amount   float64  `xml:",chardata"`
}

type Drug struct {
	XMLName xml.Name `xml:"DRUG"`
	TMTID   string   `xml:"TMTID"`
	Amount  float64  `xml:"AMOUNT"`
	Unit    string   `xml:"UNIT,omitempty"`
	Price   float64  `xml:"PRICE"`
	Cost    float64  `xml:"COST"`
}

// Build renders the CSOP XML document as bytes.
func Build(in Input) ([]byte, error) {
	if in.HCode == "" {
		return nil, fmt.Errorf("csop: HCode required")
	}
	if in.Agency == "" {
		return nil, fmt.Errorf("csop: Agency required")
	}
	if in.HCodeSub == "" {
		in.HCodeSub = in.HCode
	}
	if in.SendDate.IsZero() {
		in.SendDate = time.Now()
	}

	doc := Document{
		Header: Header{
			HospMain: in.HCode,
			HospSub:  in.HCodeSub,
			Agency:   string(in.Agency),
			Period:   in.Period,
			SendDate: util.FormatDate(in.SendDate),
		},
		Visits: make([]Visit, 0, len(in.OPD)),
	}
	for _, v := range in.OPD {
		doc.Visits = append(doc.Visits, mapVisit(v))
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("csop marshal: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}

func mapVisit(v model.OPDVisit) Visit {
	vi := Visit{
		HN:       v.Patient.HN,
		PID:      v.Patient.PersonID,
		SEQ:      v.SEQ,
		DateOPD:  util.FormatDate(v.DateOPD),
		TimeOPD:  v.TimeOPD,
		Clinic:   v.Clinic,
		TypeOut:  v.TypeOut,
		UUC:      util.StrOr(v.UUC, "1"),
		PermitNo: v.Patient.PermitNo,
		Total:    v.Total,
		Paid:     v.Paid,
	}
	for _, dx := range v.Diagnoses {
		vi.Diagnoses = append(vi.Diagnoses, Diag{DxType: dx.Type, Code: dx.Code})
	}
	for _, op := range v.Ops {
		vi.Procs = append(vi.Procs, Proc{
			DrOpID: op.DoctorID,
			Date:   util.FormatDate(op.Date),
			Code:   op.Code,
		})
	}
	for _, ch := range v.Charges {
		vi.Charges = append(vi.Charges, Charge{ChrgItem: ch.Code, Amount: ch.Amount})
	}
	for _, d := range v.Drugs {
		vi.Drugs = append(vi.Drugs, Drug{
			TMTID: d.TMTID, Amount: d.Amount, Unit: d.Unit,
			Price: d.Price, Cost: d.Cost,
		})
	}
	return vi
}

