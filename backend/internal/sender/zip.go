package sender

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"fmt"
	"sort"
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

// BuildMultiFileZip packs ≥1 files into one ZIP archive. MD5 of the original
// content is appended to each file (on a new line) before it is added to the
// archive — same convention as BuildZipWithMD5 but for the 16-file format
// where one submission carries many .txt files.
//
// Files are zipped in the order given by keys (sorted) so the archive byte
// output is deterministic; callers can hash the result for idempotency checks.
func BuildMultiFileZip(files map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(files))
	for k := range files {
		names = append(names, k)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range names {
		content := files[name]
		hash := fmt.Sprintf("%x", md5.Sum(content))
		withHash := append(append([]byte{}, content...), []byte("\n"+hash)...)
		f, err := w.Create(name)
		if err != nil {
			return nil, fmt.Errorf("create zip entry %s: %w", name, err)
		}
		if _, err := f.Write(withHash); err != nil {
			return nil, fmt.Errorf("write zip entry %s: %w", name, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}
