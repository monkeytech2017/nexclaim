// Package response parse ผลการตอบกลับจาก FDH และ CHI
package response

import (
	"encoding/xml"
	"fmt"
)

// CCodeItem รายการ C-code จาก REP
type CCodeItem struct {
	CCode     string `xml:"CCODE"`
	CDesc     string `xml:"CDESC"`
	HN        string `xml:"HN"`
	AN        string `xml:"AN"`
	SEQ       string `xml:"SEQ"`
	FieldName string `xml:"FIELDNAME"`
	FieldValue string `xml:"FIELDVALUE"`
}

// REPDocument โครงสร้าง REP XML จาก FDH
type REPDocument struct {
	XMLName xml.Name    `xml:"REP"`
	Items   []CCodeItem `xml:"ITEM"`
}

// ParseREP parse XML REP จาก FDH/CHI → รายการ C-code
func ParseREP(data []byte) ([]CCodeItem, error) {
	var doc REPDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse REP: %w", err)
	}
	return doc.Items, nil
}
