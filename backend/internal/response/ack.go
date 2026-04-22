package response

import (
	"encoding/xml"
	"fmt"
)

// ACKDocument โครงสร้าง ACK จาก FDH (ตอบรับทันที)
type ACKDocument struct {
	XMLName xml.Name `xml:"ACK"`
	Status  string   `xml:"STATUS"`
	TxnID   string   `xml:"TXNID"`
	Message string   `xml:"MESSAGE"`
}

// ParseACK parse ACK XML จาก FDH
func ParseACK(data []byte) (*ACKDocument, error) {
	var ack ACKDocument
	if err := xml.Unmarshal(data, &ack); err != nil {
		return nil, fmt.Errorf("parse ACK: %w", err)
	}
	return &ack, nil
}
