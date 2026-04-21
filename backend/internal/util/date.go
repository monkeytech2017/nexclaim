// Package util ฟังก์ชัน utility ที่ใช้ร่วมกันทั้งระบบ
package util

import (
	"fmt"
	"strconv"
	"time"
)

// ToAD แปลง พ.ศ. → ค.ศ. (HIS ไทยมักเก็บ พ.ศ.)
// ถ้าปี > 2400 ถือว่าเป็น พ.ศ. แล้วลบ 543
func ToAD(s string) string {
	if len(s) < 4 {
		return s
	}
	year, err := strconv.Atoi(s[:4])
	if err != nil {
		return s
	}
	if year > 2400 {
		return strconv.Itoa(year-543) + s[4:]
	}
	return s
}

// FormatDate → YYYYMMDD (ค.ศ.)
func FormatDate(t time.Time) string { return t.Format("20060102") }

// FormatDateTime → YYYYMMDDHHMMSS
func FormatDateTime(t time.Time) string { return t.Format("20060102150405") }

// FormatTime → HHMM
func FormatTime(t time.Time) string { return t.Format("1504") }

// ParseHISDate parse date string จาก HIS (รองรับ พ.ศ. และ ค.ศ.)
func ParseHISDate(s string) (time.Time, error) {
	s = ToAD(s)
	for _, f := range []string{"20060102", "2006-01-02", "02/01/2006"} {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q", s)
}
