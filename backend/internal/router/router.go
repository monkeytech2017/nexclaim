// Package router ตรวจสอบ INSCL แล้ว route ไปยัง format และช่องทางที่ถูกต้อง
package router

import (
	"fmt"

	"github.com/nexclaim/nexclaim/internal/model"
)

// Route รับ INSCL + ประเภทผู้ป่วย → คืน RouteResult
func Route(inscl model.INSCL, isIPD bool) (model.RouteResult, error) {
	switch inscl {

	case model.INSCL_CSMBS, model.INSCL_WEL:
		if isIPD {
			return model.RouteResult{Format: model.FormatCIPN, Agency: model.AgencyCGD, Sender: model.SenderFDH}, nil
		}
		return model.RouteResult{Format: model.FormatCSOP, Agency: model.AgencyCGD, Sender: model.SenderFDH}, nil

	case model.INSCL_LGO:
		if isIPD {
			return model.RouteResult{Format: model.FormatCIPN, Agency: model.AgencyLGO, Sender: model.SenderFDH}, nil
		}
		return model.RouteResult{Format: model.FormatCSOP, Agency: model.AgencyLGO, Sender: model.SenderFDH}, nil

	case model.INSCL_OFC:
		// Agency ดึงแยกจาก patient.AgencyCode
		if isIPD {
			return model.RouteResult{Format: model.FormatCIPN, Sender: model.SenderFDH}, nil
		}
		return model.RouteResult{Format: model.FormatCSOP, Sender: model.SenderFDH}, nil

	case model.INSCL_UCS, model.INSCL_NON, model.INSCL_WP1, model.INSCL_WP2, model.INSCL_MON:
		return model.RouteResult{Format: model.Format16Files, Sender: model.SenderFDH}, nil

	case model.INSCL_SSS, model.INSCL_SS4:
		if isIPD {
			return model.RouteResult{Format: model.FormatAIPN, Sender: model.SenderCHI}, nil
		}
		return model.RouteResult{Format: model.FormatSSOP, Sender: model.SenderCHI}, nil

	case model.INSCL_TPBS:
		return model.RouteResult{Format: model.Format16Files, Sender: model.SenderFDH}, nil
	case model.INSCL_WK:
		return model.RouteResult{Format: model.Format16Files, Sender: model.SenderWCF}, nil
	case model.INSCL_PRS:
		return model.RouteResult{Format: model.Format16Files, Sender: model.SenderDOC}, nil

	default:
		return model.RouteResult{Format: model.Format16Files, Sender: model.SenderFDH},
			fmt.Errorf("unknown INSCL %q: routed to 16files/FDH as fallback", inscl)
	}
}

// AgencyFromCode แปลง agency code string → Agency type
func AgencyFromCode(code string) model.Agency {
	m := map[string]model.Agency{
		"NBTC": model.AgencyNBTC,
		"BAAC": model.AgencyBAAC,
		"ECT":  model.AgencyECT,
		"PEA":  model.AgencyPEA,
		"MEA":  model.AgencyMEA,
		"MWA":  model.AgencyMWA,
		"SRT":  model.AgencySRT,
		"LGO":  model.AgencyLGO,
	}
	if a, ok := m[code]; ok {
		return a
	}
	return model.AgencyCGD
}
