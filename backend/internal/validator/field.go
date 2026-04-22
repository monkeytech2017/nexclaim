// Package validator ตรวจสอบข้อมูลตาม spec ของแต่ละหน่วยงาน
package validator

import (
	"fmt"
	"regexp"
	"time"
)

// IsValidDate ตรวจสอบ format YYYYMMDD ค.ศ.
func IsValidDate(s string) bool {
	_, err := time.Parse("20060102", s)
	return err == nil
}

// IsValidPersonID ตรวจสอบเลขบัตรประชาชน 13 หลัก + check digit
func IsValidPersonID(id string) bool {
	if len(id) != 13 {
		return false
	}
	matched, _ := regexp.MatchString(`^\d{13}$`, id)
	if !matched {
		return false
	}
	return verifyCheckDigit(id)
}

func verifyCheckDigit(id string) bool {
	sum := 0
	for i := 0; i < 12; i++ {
		sum += int(id[i]-'0') * (13 - i)
	}
	check := (11 - (sum % 11)) % 10
	return check == int(id[12]-'0')
}

// IsValidAN ตรวจสอบ AN: ≤9 หลัก ไม่มีอักขระพิเศษ
func IsValidAN(an string) bool {
	if len(an) == 0 || len(an) > 9 {
		return false
	}
	matched, _ := regexp.MatchString(`^[^\\/:*?"<>|]+$`, an)
	return matched
}

// IsValidUUC ตรวจสอบ UUC ต้องเป็น "1" เสมอ
func IsValidUUC(uuc string) bool {
	return uuc == "1"
}

// ValidationError ข้อผิดพลาดการ validate พร้อม field และ C-code
type ValidationError struct {
	Field  string
	Value  string
	CCode  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Errorf("%s (field=%s value=%s)", e.Reason, e.Field, e.Value).Error()
}
