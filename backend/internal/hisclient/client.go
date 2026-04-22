package hisclient

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// Client HIS API client. Auth = Bearer token OR X-API-Key (both supported).
type Client struct {
	BaseURL string
	http    *resty.Client
}

// Option knobs for the client.
type Option func(*Client)

// WithBearerToken ตั้ง Authorization: Bearer {token} ทุก request.
func WithBearerToken(token string) Option {
	return func(c *Client) { c.http.SetAuthToken(token) }
}

// WithAPIKey ตั้ง X-API-Key header (alternative auth).
func WithAPIKey(key string) Option {
	return func(c *Client) { c.http.SetHeader("X-API-Key", key) }
}

// WithTimeout override default 30s timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.SetTimeout(d) }
}

// WithHTTPClient inject resty (for tests using httptest).
func WithHTTPClient(r *resty.Client) Option {
	return func(c *Client) { c.http = r }
}

func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		http:    resty.New().SetTimeout(30 * time.Second),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// GetVisit = P0 endpoint: ดึง detail 1 visit.
func (c *Client) GetVisit(vn string) (*VisitDetail, error) {
	if vn == "" {
		return nil, fmt.Errorf("hisclient: vn required")
	}
	var out VisitDetail
	resp, err := c.http.R().
		SetResult(&out).
		Get(c.BaseURL + "/api/nexclaim/opd/visit/" + vn)
	if err != nil {
		return nil, fmt.Errorf("hisclient get visit %s: %w", vn, err)
	}
	if resp.IsError() {
		return nil, httpError("get visit "+vn, resp)
	}
	return &out, nil
}

// GetVisitsBatch = P1 endpoint: ดึงหลาย visit ในครั้งเดียว (max 50).
func (c *Client) GetVisitsBatch(vns []string) (*BatchVisitsResponse, error) {
	if len(vns) == 0 {
		return nil, fmt.Errorf("hisclient: vns required")
	}
	if len(vns) > 50 {
		return nil, fmt.Errorf("hisclient: max 50 vns per batch")
	}
	var out BatchVisitsResponse
	resp, err := c.http.R().
		SetQueryParam("vn", strings.Join(vns, ",")).
		SetResult(&out).
		Get(c.BaseURL + "/api/nexclaim/opd/visits")
	if err != nil {
		return nil, fmt.Errorf("hisclient get visits batch: %w", err)
	}
	if resp.IsError() {
		return nil, httpError("get visits batch", resp)
	}
	return &out, nil
}

// GetDrugList = P1 endpoint สำหรับ TMT mapping.
func (c *Client) GetDrugList(page, limit int) (*DrugListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 500
	}
	var out DrugListResponse
	resp, err := c.http.R().
		SetQueryParam("page", fmt.Sprintf("%d", page)).
		SetQueryParam("limit", fmt.Sprintf("%d", limit)).
		SetResult(&out).
		Get(c.BaseURL + "/api/nexclaim/drug/list")
	if err != nil {
		return nil, fmt.Errorf("hisclient drug list: %w", err)
	}
	if resp.IsError() {
		return nil, httpError("drug list", resp)
	}
	return &out, nil
}

// Health = P0 endpoint — simple connectivity check.
func (c *Client) Health() (*HealthResponse, error) {
	var out HealthResponse
	resp, err := c.http.R().
		SetResult(&out).
		Get(c.BaseURL + "/api/nexclaim/health")
	if err != nil {
		return nil, fmt.Errorf("hisclient health: %w", err)
	}
	if resp.IsError() {
		return nil, httpError("health", resp)
	}
	return &out, nil
}

func httpError(op string, resp *resty.Response) error {
	return fmt.Errorf("hisclient %s: %s: %s", op, resp.Status(), resp.String())
}
