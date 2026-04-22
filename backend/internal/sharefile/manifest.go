// Package sharefile = IPD Share Folder ingestion ตาม spec:
// HIS วาง CSV 10 ไฟล์ + MANIFEST.json ใน /shared/nexclaim/ipd/incoming/
// — NexClaim อ่าน MANIFEST.json เป็น signal แล้ว parse ไฟล์ CSV ทั้งหมด.
package sharefile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Manifest = /shared/nexclaim/ipd/incoming/{export}/MANIFEST.json
type Manifest struct {
	ExportID        string   `json:"export_id"`
	ExportDate      string   `json:"export_date"` // ISO 8601
	Period          string   `json:"period"`      // YYYYMM
	HospitalCode    string   `json:"hospital_code"`
	TotalAdmissions int      `json:"total_admissions"`
	Files           []string `json:"files"`
	ExportedBy      string   `json:"exported_by,omitempty"`
}

// LoadManifest อ่าน MANIFEST.json จาก folder ที่กำหนด.
func LoadManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, "MANIFEST.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate ensures the manifest declares required fields + the mandatory
// CSV files (PAT, IPD). Existence of each file on disk is left to Parse —
// opening them there produces a single source of truth for I/O errors.
func (m *Manifest) Validate() error {
	if m.ExportID == "" || m.Period == "" || m.HospitalCode == "" {
		return fmt.Errorf("manifest: export_id, period, hospital_code required")
	}
	if len(m.Files) == 0 {
		return fmt.Errorf("manifest: files[] required")
	}
	declared := make(map[string]bool, len(m.Files))
	for _, f := range m.Files {
		declared[f] = true
	}
	for _, req := range RequiredFiles {
		if !declared[req] {
			return fmt.Errorf("manifest: required file %s not declared", req)
		}
	}
	return nil
}

// ExportTime parses ExportDate (ISO 8601) or returns zero time.
func (m *Manifest) ExportTime() time.Time {
	t, _ := time.Parse(time.RFC3339, m.ExportDate)
	return t
}
