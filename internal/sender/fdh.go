// Package sender ส่งข้อมูลไปยังกองทุนต่างๆ
package sender

import "fmt"

// FDHClient ส่งข้อมูลผ่าน MOPH Financial Data Hub
type FDHClient struct {
	BaseURL  string
	Username string
	Password string
	HCode    string
}

func NewFDHClient(baseURL, username, password, hcode string) *FDHClient {
	return &FDHClient{BaseURL: baseURL, Username: username, Password: password, HCode: hcode}
}

func (c *FDHClient) String() string {
	return fmt.Sprintf("FDHClient{hcode=%s url=%s}", c.HCode, c.BaseURL)
}

// TODO หลัง go mod tidy:
// - GetToken() (string, error)
// - Send16Files(zipData []byte, period string) (string, error)
// - SendCIPN(zipData []byte, period string) (string, error)
// - SendCSOP(zipData []byte, period string) (string, error)
// - GetStatus(txnID string) (string, error)
// - GetREP(period string) ([]byte, error)
