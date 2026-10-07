package rocnovel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"go_backend/internal/config"
)

type Client struct {
	httpClient *http.Client
	cfg        *config.RocnovelAPI
}

func NewClient(cfg *config.RocnovelAPI) *Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   45 * time.Second,
		},
		cfg: cfg,
	}
}

type OrderReportRequest struct {
	ContentType   int    `json:"contentType"`
	ClientGroupID string `json:"clientGroupId"`
	PageIndex     int    `json:"pageIndex"`
	PageSize      int    `json:"pageSize"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	PId           string `json:"pId"`
	Timestamp     int64  `json:"_"`
}

type OrderReportResponse struct {
	Code    int              `json:"code"`
	Msg     string           `json:"msg"`
	Success bool             `json:"success"`
	Data    *OrderReportData `json:"data"`
}

type OrderReportData struct {
	Total       int64               `json:"total"`
	CurrentPage int                 `json:"currentPage"`
	PageSize    int                 `json:"pageSize"`
	Pages       int                 `json:"pages"`
	Records     []OrderReportRecord `json:"records"`
}

type OrderReportRecord struct {
	OrderID         string `json:"orderId"`
	MemberID        string `json:"memberId"`
	LandingPageID   string `json:"PId"`
	UserCreateTime  string `json:"userCreateTime"` // 北京时间 yyyy-MM-dd HH:mm:ss
	PayDate         string `json:"payDate"`        // 北京时间 yyyy-MM-dd HH:mm:ss
	OrderAmountCent int    `json:"orderAmount"`    // 分
	OrderAmountUSD  string `json:"orderAmountUsd"` // 美元
	IsSubs          int    `json:"isSubs"`         // 是否订阅: 1-是, 0-否
	RenewType       int    `json:"renewType"`      // 1-首充, 2-自动续费
	PayState        int    `json:"payState"`       // 1-成功
	RefundStatus    int    `json:"refundStatus"`   // 0-未退款, 1-部分退款, 2-已退款
}

type LandingPageRecordDto struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	ContentID           string `json:"contentId"`
	ContentName         string `json:"contentName"`
	SaleComboID         string `json:"saleComboId"`
	SaleComboName       string `json:"saleComboName"`
	SubscribeConfigID   string `json:"subscribeConfigId"`
	SubscribeConfigName string `json:"subscribeConfigName"`
	CreateDateTime      string `json:"createDateTime"`
	UpdateDateTime      string `json:"updateDateTime"`
}

type LandingPageConfigResponseDto struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data *struct {
		Current int                    `json:"current"`
		Pages   int                    `json:"pages"`
		Size    int                    `json:"size"`
		Total   int                    `json:"total"`
		Records []LandingPageRecordDto `json:"records"`
	} `json:"data"`
}

type SubscribeConfigProductRecordDto struct {
	ID                string `json:"id"`
	ConfigID          string `json:"configId"`
	Name              string `json:"name"`
	Cycle             int    `json:"cycle"`
	CycleStr          string `json:"cycleStr"`
	PreferentialPrice string `json:"preferentialPrice"`
	Price             string `json:"price"`
	Status            int    `json:"status"`
	Deleted           int    `json:"deleted"`
	CreateDateTime    string `json:"createDateTime"`
	UpdateDateTime    string `json:"updateDateTime"`
}

type SubscribeConfigProductResponseDto struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data *struct {
		Current int                                `json:"current"`
		Pages   int                                `json:"pages"`
		Size    int                                `json:"size"`
		Total   int                                `json:"total"`
		Records []SubscribeConfigProductRecordDto `json:"records"`
	} `json:"data"`
}

func (c *Client) FetchOrdersPage(ctx context.Context, pageIndex, pageSize int, startTime, endTime, pID, auth, cookie string) (*OrderReportData, error) {
	reqBody := OrderReportRequest{
		ContentType:   4,
		ClientGroupID: c.cfg.ClientGroupID,
		PageIndex:     pageIndex,
		PageSize:      pageSize,
		StartTime:     startTime,
		EndTime:       endTime,
		PId:           pID,
		Timestamp:     time.Now().UnixMilli(),
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	var respBytes []byte
	var lastErr error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.URL, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "*/*")

		activeAuth := strings.TrimSpace(auth)
		if activeAuth == "" {
			activeAuth = c.cfg.Authorization
		}
		if activeAuth != "" {
			req.Header.Set("Authorization", activeAuth)
		}

		activeCookie := strings.TrimSpace(cookie)
		if activeCookie == "" {
			activeCookie = c.cfg.Cookie
		}
		if activeCookie != "" {
			req.Header.Set("Cookie", activeCookie)
		}

		if c.cfg.ClientGroupID != "" {
			req.Header.Set("client-group-id", c.cfg.ClientGroupID)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			time.Sleep(time.Duration(attempt*300) * time.Millisecond)
			continue
		}

		b, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			time.Sleep(time.Duration(attempt*300) * time.Millisecond)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("http error code: %d, body: %s", resp.StatusCode, string(b))
			if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				time.Sleep(time.Duration(attempt*300) * time.Millisecond)
				continue
			}
			return nil, lastErr
		}

		respBytes = b
		lastErr = nil
		break
	}

	if lastErr != nil {
		return nil, fmt.Errorf("do request failed: %w", lastErr)
	}

	var res OrderReportResponse
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	if res.Code == 4002 {
		return nil, fmt.Errorf("TOKEN_EXPIRED: 登录 Token 已过期，请在页面更新最新 Token")
	}

	if !res.Success && res.Code != 200 && res.Code != 0 {
		return nil, fmt.Errorf("api error code: %d, msg: %s", res.Code, res.Msg)
	}

	if res.Data == nil {
		return &OrderReportData{Records: []OrderReportRecord{}}, nil
	}

	return res.Data, nil
}

// FetchLandingPagesPage 查询中文在线落地页列表
func (c *Client) FetchLandingPagesPage(ctx context.Context, pageIndex, pageSize int, auth, cookie string) (*LandingPageConfigResponseDto, error) {
	clientGroupID := c.cfg.ClientGroupID
	if clientGroupID == "" {
		clientGroupID = "405323222546395136"
	}
	url := fmt.Sprintf("https://admin-api.rocnovel.com/landingPage/config/list?clientGroupId=%s&clientGroupName=Florastory&contentType=4&pageIndex=%d&pageSize=%d&_=%d",
		clientGroupID, pageIndex, pageSize, time.Now().UnixMilli())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36")
	activeAuth := strings.TrimSpace(auth)
	if activeAuth == "" {
		activeAuth = c.cfg.Authorization
	}
	if activeAuth != "" {
		req.Header.Set("Authorization", activeAuth)
	}
	activeCookie := strings.TrimSpace(cookie)
	if activeCookie == "" {
		activeCookie = c.cfg.Cookie
	}
	if activeCookie != "" {
		req.Header.Set("Cookie", activeCookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var res LandingPageConfigResponseDto
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal landing page response failed: %w", err)
	}
	return &res, nil
}

// FetchSubscribeProductsForConfig 查询指定订阅配置下的产品明细
func (c *Client) FetchSubscribeProductsForConfig(ctx context.Context, configID, auth, cookie string) ([]SubscribeConfigProductRecordDto, error) {
	clientGroupID := c.cfg.ClientGroupID
	if clientGroupID == "" {
		clientGroupID = "405323222546395136"
	}
	url := fmt.Sprintf("https://admin-api.rocnovel.com/subscribe-config/product/list?clientGroupId=%s&clientGroupName=Florastory&configId=%s&contentType=4&pageIndex=1&pageSize=1000&paymentMethod=3",
		clientGroupID, configID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36")
	activeAuth := strings.TrimSpace(auth)
	if activeAuth == "" {
		activeAuth = c.cfg.Authorization
	}
	if activeAuth != "" {
		req.Header.Set("Authorization", activeAuth)
	}
	activeCookie := strings.TrimSpace(cookie)
	if activeCookie == "" {
		activeCookie = c.cfg.Cookie
	}
	if activeCookie != "" {
		req.Header.Set("Cookie", activeCookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var res SubscribeConfigProductResponseDto
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal product response failed: %w", err)
	}
	if res.Data != nil && res.Data.Records != nil {
		return res.Data.Records, nil
	}
	return []SubscribeConfigProductRecordDto{}, nil
}

