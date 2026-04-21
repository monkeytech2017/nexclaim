package sender

import "fmt"

// CHIClient ส่งข้อมูล SSO ผ่าน cs8.chi.or.th
type CHIClient struct {
	BaseURL  string
	Username string
	Password string
}

func NewCHIClient(baseURL, username, password string) *CHIClient {
	return &CHIClient{BaseURL: baseURL, Username: username, Password: password}
}

func (c *CHIClient) String() string {
	return fmt.Sprintf("CHIClient{url=%s}", c.BaseURL)
}

// TODO: SendSSOPZip, SendAIPNZip
