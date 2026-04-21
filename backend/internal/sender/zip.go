package sender

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"fmt"
)

// BuildZipWithMD5 สร้าง ZIP file + append MD5 hash ท้ายไฟล์ XML
// ตาม spec: HCODE + FORMAT + YYYYMM.ZIP เช่น 12345CIPN202504.ZIP
func BuildZipWithMD5(xmlContent []byte, filename string) ([]byte, error) {
	// คำนวณ MD5 ของ XML content
	hash := fmt.Sprintf("%x", md5.Sum(xmlContent))

	// append MD5 ต่อท้าย XML (ก่อน zip)
	withHash := append(xmlContent, []byte("\n"+hash)...)

	// สร้าง ZIP
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("create zip entry %s: %w", filename, err)
	}
	if _, err := f.Write(withHash); err != nil {
		return nil, fmt.Errorf("write zip entry: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	return buf.Bytes(), nil
}

// ZipFilename สร้างชื่อไฟล์ ZIP ตาม convention
// format: {HCODE}{FORMAT}{YYYYMM}.ZIP
func ZipFilename(hcode, format, period string) string {
	return fmt.Sprintf("%s%s%s.ZIP", hcode, format, period)
}
