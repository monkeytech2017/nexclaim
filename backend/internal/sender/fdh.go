// Package sender ส่งข้อมูลไปยังกองทุนต่างๆ
package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// FDHClient ส่งข้อมูลผ่าน MOPH Financial Data Hub
//
// Thread-safe: token refresh ถูก serialize ด้วย mutex; หนึ่ง client ใช้ซ้ำได้
// หลาย goroutine. Token ถูก cache ไว้จนใกล้หมดอายุ (buffer 60 วินาที)
// เพื่อลด auth roundtrip.
type FDHClient struct {
	BaseURL  string
	Username string
	Password string
	HCode    string

	http *resty.Client

	mu         sync.Mutex
	token      string
	tokenExp   time.Time
}

// FDHOption allows callers to inject a custom resty client (e.g. for tests
// that point at a mock server or need a short timeout).
type FDHOption func(*FDHClient)

func WithHTTPClient(r *resty.Client) FDHOption {
	return func(c *FDHClient) { c.http = r }
}

func NewFDHClient(baseURL, username, password, hcode string, opts ...FDHOption) *FDHClient {
	c := &FDHClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		HCode:    hcode,
		http:     resty.New().SetTimeout(60 * time.Second),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *FDHClient) String() string {
	return fmt.Sprintf("FDHClient{hcode=%s url=%s}", c.HCode, c.BaseURL)
}

// ── Auth ──────────────────────────────────────────────────────

type fdhTokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // seconds
	TokenType   string `json:"token_type"`
}

// GetToken returns a valid Bearer token, refreshing if expired.
func (c *FDHClient) GetToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Until(c.tokenExp) > time.Minute {
		return c.token, nil
	}
	var out fdhTokenResp
	resp, err := c.http.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]string{
			"username": c.Username,
			"password": c.Password,
			"hcode":    c.HCode,
		}).
		SetResult(&out).
		Post(c.BaseURL + "/api/auth/token")
	if err != nil {
		return "", fmt.Errorf("fdh auth: %w", err)
	}
	if resp.IsError() {
		return "", fmt.Errorf("fdh auth: %s: %s", resp.Status(), resp.String())
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("fdh auth: empty access_token")
	}
	c.token = out.AccessToken
	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	c.tokenExp = time.Now().Add(ttl)
	return c.token, nil
}

// ── Submit ────────────────────────────────────────────────────

// SubmitResult is the ACK body returned by FDH upload endpoints.
type SubmitResult struct {
	TxnID   string `json:"txnId"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Send16Files POST ZIP → /api/claim/16files (multipart)
func (c *FDHClient) Send16Files(zipData []byte, period string) (*SubmitResult, error) {
	return c.upload("/api/claim/16files", zipData, period)
}

// SendCIPN POST ZIP → /api/claim/cipn
func (c *FDHClient) SendCIPN(zipData []byte, period string) (*SubmitResult, error) {
	return c.upload("/api/claim/cipn", zipData, period)
}

// SendCSOP POST ZIP → /api/claim/csop
func (c *FDHClient) SendCSOP(zipData []byte, period string) (*SubmitResult, error) {
	return c.upload("/api/claim/csop", zipData, period)
}

func (c *FDHClient) upload(path string, zipData []byte, period string) (*SubmitResult, error) {
	token, err := c.GetToken()
	if err != nil {
		return nil, err
	}
	var out SubmitResult
	resp, err := c.http.R().
		SetAuthToken(token).
		SetFileReader("file", c.HCode+"_"+period+".ZIP", bytes.NewReader(zipData)).
		SetFormData(map[string]string{
			"hcode":  c.HCode,
			"period": period,
		}).
		SetResult(&out).
		Post(c.BaseURL + path)
	if err != nil {
		return nil, fmt.Errorf("fdh %s: %w", path, err)
	}
	if resp.IsError() {
		// try to decode error body, fall back to raw
		var errOut struct{ Message string `json:"message"` }
		_ = json.Unmarshal(resp.Body(), &errOut)
		msg := errOut.Message
		if msg == "" {
			msg = resp.String()
		}
		return nil, fmt.Errorf("fdh %s: %s: %s", path, resp.Status(), msg)
	}
	return &out, nil
}

// ── Status / REP ──────────────────────────────────────────────

// GetStatus GET /api/claim/status/{txnId}
func (c *FDHClient) GetStatus(txnID string) (*SubmitResult, error) {
	token, err := c.GetToken()
	if err != nil {
		return nil, err
	}
	var out SubmitResult
	resp, err := c.http.R().
		SetAuthToken(token).
		SetResult(&out).
		Get(c.BaseURL + "/api/claim/status/" + txnID)
	if err != nil {
		return nil, fmt.Errorf("fdh status: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("fdh status: %s: %s", resp.Status(), resp.String())
	}
	return &out, nil
}

// GetREP downloads the REP (C-code) XML for a given period.
func (c *FDHClient) GetREP(period string) ([]byte, error) {
	token, err := c.GetToken()
	if err != nil {
		return nil, err
	}
	resp, err := c.http.R().
		SetAuthToken(token).
		Get(c.BaseURL + "/api/claim/rep/" + period)
	if err != nil {
		return nil, fmt.Errorf("fdh rep: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("fdh rep: %s: %s", resp.Status(), resp.String())
	}
	return resp.Body(), nil
}
