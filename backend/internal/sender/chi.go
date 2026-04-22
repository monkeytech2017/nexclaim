package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

// CHIClient ส่งข้อมูล SSO ผ่าน cs8.chi.or.th
//
// SSOP endpoint: /ssopupload/ (ปกติ OPD ม.33/39/40)
// AIPN endpoint: /aipnupload/ (IPD)
// Authentication: HTTP Basic (username/password) — ไม่ใช้ JWT เหมือน FDH
type CHIClient struct {
	BaseURL  string
	Username string
	Password string
	HCode    string
	http     *resty.Client
}

type CHIOption func(*CHIClient)

func WithCHIHTTPClient(r *resty.Client) CHIOption {
	return func(c *CHIClient) { c.http = r }
}

func NewCHIClient(baseURL, username, password string, opts ...CHIOption) *CHIClient {
	c := &CHIClient{
		BaseURL: baseURL, Username: username, Password: password,
		http: resty.New().SetTimeout(60 * time.Second),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// WithHCode attaches an HCode to the client (used in filenames); returns c for chaining.
func (c *CHIClient) WithHCode(hcode string) *CHIClient {
	c.HCode = hcode
	return c
}

func (c *CHIClient) String() string {
	return fmt.Sprintf("CHIClient{url=%s hcode=%s}", c.BaseURL, c.HCode)
}

// SendAIPN uploads an AIPN zip (IPD) to /aipnupload/.
func (c *CHIClient) SendAIPN(zipData []byte, period string) (*SubmitResult, error) {
	return c.upload("/aipnupload/", zipData, period)
}

// SendSSOP uploads an SSOP zip (OPD) to /ssopupload/.
func (c *CHIClient) SendSSOP(zipData []byte, period string) (*SubmitResult, error) {
	return c.upload("/ssopupload/", zipData, period)
}

func (c *CHIClient) upload(path string, zipData []byte, period string) (*SubmitResult, error) {
	filename := c.HCode + "_" + period + ".ZIP"
	var out SubmitResult
	req := c.http.R().
		SetBasicAuth(c.Username, c.Password).
		SetFileReader("file", filename, bytes.NewReader(zipData)).
		SetFormData(map[string]string{
			"hcode":  c.HCode,
			"period": period,
		}).
		SetResult(&out)
	resp, err := req.Post(c.BaseURL + path)
	if err != nil {
		return nil, fmt.Errorf("chi %s: %w", path, err)
	}
	if resp.IsError() {
		var errOut struct{ Message string `json:"message"` }
		_ = json.Unmarshal(resp.Body(), &errOut)
		msg := errOut.Message
		if msg == "" {
			msg = resp.String()
		}
		return nil, fmt.Errorf("chi %s: %s: %s", path, resp.Status(), msg)
	}
	return &out, nil
}
