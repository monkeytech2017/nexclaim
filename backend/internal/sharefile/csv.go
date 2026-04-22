package sharefile

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ReadPipeCSV อ่าน CSV แบบ pipe-delimited, UTF-8, มี header row.
// คืน []map[column]value (ไม่ parse type).
// Empty string ใน field = NULL/missing ตาม spec (ไม่ใช้ NULL literal).
//
// ไม่รองรับ quoted fields ที่มี pipe ข้างใน (HIS ต้อง escape ถ้าจำเป็น —
// แต่สเปคระบุ "ชื่อยาอาจมี comma" เป็นเหตุผลที่เลือก pipe).
func ReadPipeCSV(dir, name string) ([]map[string]string, error) {
	path := filepath.Join(dir, name)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return parsePipeCSV(f, name)
}

func parsePipeCSV(r io.Reader, source string) ([]map[string]string, error) {
	cr := csv.NewReader(bufio.NewReader(r))
	cr.Comma = '|'
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1 // allow variable — we'll check against header length
	cr.ReuseRecord = false

	header, err := cr.Read()
	if err == io.EOF {
		return nil, nil // empty file = valid (e.g. optional CSVs)
	}
	if err != nil {
		return nil, fmt.Errorf("%s header: %w", source, err)
	}
	for i, h := range header {
		header[i] = strings.TrimSpace(h)
	}

	var rows []map[string]string
	lineNo := 1
	for {
		rec, err := cr.Read()
		lineNo++
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", source, lineNo, err)
		}
		if len(rec) == 1 && rec[0] == "" {
			continue // skip blank lines
		}
		m := make(map[string]string, len(header))
		for i, col := range header {
			if i < len(rec) {
				m[col] = rec[i]
			} else {
				m[col] = ""
			}
		}
		rows = append(rows, m)
	}
	return rows, nil
}
