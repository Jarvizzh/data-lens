package rocnovel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go_backend/internal/config"
)

type Client struct {
	httpClient *http.Client
	cfg        *config.RocnovelAPI
}

func NewClient(cfg *config.RocnovelAPI) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cfg:        cfg,
	}
}

type OrderReportRequest struct {
	PageIndex int    `json:"pageIndex"`
	PageSize  int    `json:"pageSize"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type OrderReportResponse struct {
	Code    int               `json:"code"`
	Msg     string            `json:"msg"`
	Success bool              `json:"success"`
	Data    *OrderReportData  `json:"data"`
}

type OrderReportData struct {
	Total       int64               `json:"total"`
	CurrentPage int                 `json:"currentPage"`
	PageSize    int                 `json:"pageSize"`
	Records     []OrderReportRecord `json:"records"`
}

type OrderReportRecord struct {
	OrderNo          string `json:"orderNo"`
	MemberID         string `json:"memberId"`
	LandingPageID    string `json:"landingPageId"`
	RegisterTime     string `json:"registerTime"`     // 北京时间 yyyy-MM-dd HH:mm:ss
	PayTime          string `json:"payTime"`          // 北京时间 yyyy-MM-dd HH:mm:ss
	OrderAmountCent  int    `json:"orderAmount"`      // 分
	OrderAmountUSD   string `json:"orderAmountUsd"`   // 美元
	IsSubs           int    `json:"isSubs"`           // 是否订阅: 1-是, 0-否
	RenewType        int    `json:"renewType"`        // 1-首充, 2-自动续费
	PayState         int    `json:"payState"`         // 1-成功
	RefundStatus     int    `json:"refundStatus"`     // 0-未退款, 1-已退款
}

func (c *Client) FetchOrdersPage(ctx context.Context, pageIndex, pageSize int, startTime, endTime string) (*OrderReportData, error) {
	reqBody := OrderReportRequest{
		PageIndex: pageIndex,
		PageSize:  pageSize,
		StartTime: startTime,
		EndTime:   endTime,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.Authorization != "" {
		req.Header.Set("Authorization", c.cfg.Authorization)
	}
	if c.cfg.Cookie != "" {
		req.Header.Set("Cookie", c.cfg.Cookie)
	}
	if c.cfg.ClientGroupID != "" {
		req.Header.Set("client-group-id", c.cfg.ClientGroupID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error code: %d, body: %s", resp.StatusCode, string(respBytes))
	}

	var res OrderReportResponse
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	if !res.Success && res.Code != 200 && res.Code != 0 {
		return nil, fmt.Errorf("api error code: %d, msg: %s", res.Code, res.Msg)
	}

	if res.Data == nil {
		return &OrderReportData{Records: []OrderReportRecord{}}, nil
	}

	return res.Data, nil
}
