// Package cipn สร้าง XML CIPN (IPD ข้าราชการ/อปท./หน่วยงานอิสระ) → ส่ง FDH
//
// Element names (ADMIT, DIAG, PROC, CHARGE, DRUG, DRG ...) ตรงตาม struct tag;
// ผิดชื่อ = build error — ไม่รอ runtime.
package cipn

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/util"
)

// Input ข้อมูลที่ต้องการสร้าง CIPN หนึ่งรอบ
type Input struct {
	HCode    string       // รหัส รพ. 5 หลัก
	HCodeSub string       // รหัสสถานพยาบาลย่อย (default = HCode)
	Agency   model.Agency // CGD / LGO / NBTC / BAAC ...
	Period   string       // YYYYMM
	SendDate time.Time    // วันที่ส่ง (default = now ถ้า zero)
	IPD      []model.IPDAdmit
}

// Document root XML <CIPN>
type Document struct {
	XMLName xml.Name `xml:"CIPN"`
	Header  Header   `xml:"HEADER"`
	Admits  []Admit  `xml:"ADMIT"`
}

// Header metadata ของ submission
type Header struct {
	HospMain string `xml:"HMAIN"`
	HospSub  string `xml:"HSUB"`
	Agency   string `xml:"AGENCY"`
	Period   string `xml:"PERIOD"`
	SendDate string `xml:"SENDDATE"` // YYYYMMDD
}

// Admit หนึ่ง record ต่อหนึ่ง admission
type Admit struct {
	XMLName   xml.Name `xml:"ADMIT"`
	HN        string   `xml:"HN"`
	PID       string   `xml:"PID"`
	AN        string   `xml:"AN"`
	DateAdm   string   `xml:"DATEADM"`
	TimeAdm   string   `xml:"TIMEADM"`
	DateDsc   string   `xml:"DATEDSC"`
	TimeDsc   string   `xml:"TIMEDSC"`
	WardDsc   string   `xml:"WARDDSC,omitempty"`
	LOS       int      `xml:"LOS"`
	Dischs    string   `xml:"DISCHS"`
	Discht    string   `xml:"DISCHT"`
	AdmDx     string   `xml:"ADMDX,omitempty"`
	PermitNo  string   `xml:"PERMITNO,omitempty"`
	UUC       string   `xml:"UUC"`
	Diagnoses []Diag   `xml:"DIAG"`
	Procs     []Proc   `xml:"PROC"`
	Charges   []Charge `xml:"CHARGE"`
	Drugs     []Drug   `xml:"DRUG"`
	DRG       *DRGInfo `xml:"DRG,omitempty"`
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

type DRGInfo struct {
	XMLName xml.Name `xml:"DRG"`
	Code    string   `xml:"CODE"`
	Version string   `xml:"VERSION,omitempty"`
	RW      float64  `xml:"RW,omitempty"`
	AdjRW   float64  `xml:"ADJRW,omitempty"`
	LostDay int      `xml:"LOSTDAY,omitempty"`
}

// Build renders the CIPN XML document as bytes.
// Output ถูก serialized แบบ deterministic; MD5 computed downstream ใน sender.
func Build(in Input) ([]byte, error) {
	if in.HCode == "" {
		return nil, fmt.Errorf("cipn: HCode required")
	}
	if in.Agency == "" {
		return nil, fmt.Errorf("cipn: Agency required")
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
		Admits: make([]Admit, 0, len(in.IPD)),
	}

	for _, a := range in.IPD {
		doc.Admits = append(doc.Admits, mapAdmit(a))
	}

	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("cipn marshal: %w", err)
	}
	// เพิ่ม XML declaration ให้ครบ (MarshalIndent ไม่ใส่ให้)
	return append([]byte(xml.Header), out...), nil
}

func mapAdmit(a model.IPDAdmit) Admit {
	ad := Admit{
		HN:       a.Patient.HN,
		PID:      a.Patient.PersonID,
		AN:       a.AN,
		DateAdm:  util.FormatDate(a.DateAdm),
		TimeAdm:  a.TimeAdm,
		DateDsc:  util.FormatDate(a.DateDsc),
		TimeDsc:  a.TimeDsc,
		WardDsc:  a.WardDsc,
		LOS:      a.LOS,
		Dischs:   a.Dischs,
		Discht:   a.Discht,
		AdmDx:    a.AdmDx,
		PermitNo: a.Patient.PermitNo,
		UUC:      util.StrOr(a.UUC, "1"),
		Total:    a.Total,
		Paid:     a.Paid,
	}
	for _, dx := range a.Diagnoses {
		ad.Diagnoses = append(ad.Diagnoses, Diag{DxType: dx.Type, Code: dx.Code})
	}
	for _, op := range a.Ops {
		ad.Procs = append(ad.Procs, Proc{
			DrOpID: op.DoctorID,
			Date:   util.FormatDate(op.Date),
			Code:   op.Code,
		})
	}
	for _, ch := range a.Charges {
		ad.Charges = append(ad.Charges, Charge{ChrgItem: ch.Code, Amount: ch.Amount})
	}
	for _, d := range a.Drugs {
		ad.Drugs = append(ad.Drugs, Drug{
			TMTID: d.TMTID, Amount: d.Amount, Unit: d.Unit,
			Price: d.Price, Cost: d.Cost,
		})
	}
	if a.DRG != nil {
		ad.DRG = &DRGInfo{
			Code: a.DRG.Code, Version: a.DRG.Version,
			RW: a.DRG.RW, AdjRW: a.DRG.AdjRW, LostDay: a.DRG.LostDay,
		}
	}
	return ad
}

