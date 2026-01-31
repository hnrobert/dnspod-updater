package dnspod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ClientOptions struct {
	HTTPTimeout time.Duration
	BaseURL     string
	UserAgent   string
}

type Client struct {
	baseURL   string
	userAgent string
	hc        *http.Client
}

func NewClient(opt ClientOptions) *Client {
	base := strings.TrimRight(opt.BaseURL, "/")
	if base == "" {
		base = "https://dnsapi.cn"
	}
	ua := opt.UserAgent
	if ua == "" {
		ua = "dnspod-updater"
	}
	to := opt.HTTPTimeout
	if to == 0 {
		to = 10 * time.Second
	}
	return &Client{
		baseURL:   base,
		userAgent: ua,
		hc: &http.Client{
			Timeout: to,
		},
	}
}

type Status struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type RecordInfoResponse struct {
	Status Status `json:"status"`
	Record struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Value  string `json:"value"`
		Type   string `json:"type"`
		Line   string `json:"line"`
		LineID string `json:"line_id"`
		TTL    string `json:"ttl"`
		Status string `json:"status"`
	} `json:"record"`
}

type RecordModifyResponse struct {
	Status Status `json:"status"`
	Record struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Value  string `json:"value"`
		Status string `json:"status"`
	} `json:"record"`
}

func (c *Client) RecordInfo(ctx context.Context, req CommonRequest, recordID int) (RecordInfoResponse, error) {
	form := req.toForm()
	form.Set("record_id", strconv.Itoa(recordID))

	var out RecordInfoResponse
	if err := c.postForm(ctx, "/Record.Info", form, &out); err != nil {
		return RecordInfoResponse{}, err
	}
	if out.Status.Code != "1" {
		return RecordInfoResponse{}, apiError(out.Status)
	}
	return out, nil
}

type ModifyRecordParams struct {
	SubDomain    string
	RecordType   string
	RecordLine   string
	RecordLineID string
	Value        string
	MX           int
	TTL          int
	Status       string
	Weight       *int
}

func (c *Client) RecordModify(ctx context.Context, req CommonRequest, recordID int, p ModifyRecordParams) (RecordModifyResponse, error) {
	form := req.toForm()
	form.Set("record_id", strconv.Itoa(recordID))
	if p.SubDomain != "" {
		form.Set("sub_domain", p.SubDomain)
	}
	form.Set("record_type", strings.ToUpper(p.RecordType))
	if p.RecordLineID != "" {
		form.Set("record_line_id", p.RecordLineID)
	} else {
		form.Set("record_line", p.RecordLine)
	}
	form.Set("value", p.Value)
	if strings.ToUpper(p.RecordType) == "MX" {
		form.Set("mx", strconv.Itoa(p.MX))
	}
	if p.TTL > 0 {
		form.Set("ttl", strconv.Itoa(p.TTL))
	}
	if p.Status != "" {
		form.Set("status", p.Status)
	}
	if p.Weight != nil {
		form.Set("weight", strconv.Itoa(*p.Weight))
	}

	var out RecordModifyResponse
	if err := c.postForm(ctx, "/Record.Modify", form, &out); err != nil {
		return RecordModifyResponse{}, err
	}
	if out.Status.Code != "1" {
		return RecordModifyResponse{}, apiError(out.Status)
	}
	return out, nil
}

type CommonRequest struct {
	LoginToken   string
	Format       string
	Lang         string
	ErrorOnEmpty string

	Domain   string
	DomainID int
}

func (r CommonRequest) toForm() url.Values {
	v := url.Values{}
	v.Set("login_token", r.LoginToken)
	if r.Format != "" {
		v.Set("format", r.Format)
	}
	if r.Lang != "" {
		v.Set("lang", r.Lang)
	}
	if r.ErrorOnEmpty != "" {
		v.Set("error_on_empty", r.ErrorOnEmpty)
	}
	if r.DomainID != 0 {
		v.Set("domain_id", strconv.Itoa(r.DomainID))
	} else {
		v.Set("domain", r.Domain)
	}
	return v
}

type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("dnspod api error code=%s message=%s", e.Code, e.Message)
}

func apiError(s Status) error {
	return &APIError{Code: s.Code, Message: s.Message}
}

func (c *Client) postForm(ctx context.Context, path string, form url.Values, out any) error {
	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("dnspod http %d: %s", resp.StatusCode, truncate(string(b), 512))
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode response: %w (body=%s)", err, truncate(string(b), 512))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

var ErrNoChange = errors.New("no change")
