package flicknovel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go_backend/internal/config"
	"go_backend/internal/pkg/crypto"
)

type Client struct {
	httpClient *http.Client
	cfg        *config.FlicknovelAPI
}

func NewClient(cfg *config.FlicknovelAPI) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cfg:        cfg,
	}
}

func (c *Client) GetConfig() (baseURL, companyID, privateKey, defaultEmail string) {
	if c.cfg == nil {
		return "", "", "", ""
	}
	return c.cfg.BaseURL, c.cfg.CompanyID, c.cfg.PrivateKey, c.cfg.DefaultEmail
}

func (c *Client) UpdateCredentials(companyID, privateKey, defaultEmail string) {
	if c.cfg == nil {
		c.cfg = &config.FlicknovelAPI{}
	}
	if companyID != "" {
		c.cfg.CompanyID = companyID
	}
	if privateKey != "" {
		c.cfg.PrivateKey = privateKey
	}
	if defaultEmail != "" {
		c.cfg.DefaultEmail = defaultEmail
	}
}

func (c *Client) post(ctx context.Context, apiPath string, bodyObj interface{}, resultObj interface{}) error {
	bodyJSON := "{}"
	if bodyObj != nil {
		b, err := json.Marshal(bodyObj)
		if err != nil {
			return fmt.Errorf("marshal request body failed: %w", err)
		}
		bodyJSON = string(b)
	}

	timestampStr := strconv.FormatInt(time.Now().Unix(), 10)
	queryParams := map[string]string{
		"company_id": c.cfg.CompanyID,
		"timestamp":  timestampStr,
	}

	sign, err := crypto.GenerateFlicknovelSign(queryParams, bodyJSON, c.cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("generate ed25519 sign failed: %w", err)
	}
	queryParams["sign"] = sign

	// 构建 URL
	reqURL := fmt.Sprintf("%s%s", c.cfg.BaseURL, apiPath)
	u, err := url.Parse(reqURL)
	if err != nil {
		return fmt.Errorf("parse url failed: %w", err)
	}
	q := u.Query()
	for k, v := range queryParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader([]byte(bodyJSON)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status: %d, body: %s", resp.StatusCode, string(respBytes))
	}

	if err := json.Unmarshal(respBytes, resultObj); err != nil {
		return fmt.Errorf("unmarshal response failed: %w, raw: %s", err, string(respBytes))
	}

	return nil
}

// QueryPromotions 查询推广链接列表
func (c *Client) QueryPromotions(ctx context.Context, req PromotionQueryRequest) (*PromotionQueryData, error) {
	if req.DistAppID == 0 {
		req.DistAppID = c.cfg.DefaultDistAppID
	}
	var resp BaseResponse[PromotionQueryData]
	err := c.post(ctx, "/dist_app/v1/promotion/query/", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("api error code: %d, message: %s", resp.Code, resp.Message)
	}
	return &resp.Data, nil
}

// QueryRechargeTemplates 查询充值模板
func (c *Client) QueryRechargeTemplates(ctx context.Context, req RechargeTemplateQueryRequest) (*RechargeTemplateData, error) {
	if req.DistAppID == 0 {
		req.DistAppID = c.cfg.DefaultDistAppID
	}
	var resp BaseResponse[RechargeTemplateData]
	err := c.post(ctx, "/dist_app/v1/recharge_template/query/", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("api error code: %d, message: %s", resp.Code, resp.Message)
	}
	return &resp.Data, nil
}

// QueryOrders 查询订单
func (c *Client) QueryOrders(ctx context.Context, req OrderQueryRequest) (*OrderQueryData, error) {
	if req.DistAppID == 0 {
		req.DistAppID = c.cfg.DefaultDistAppID
	}
	var resp BaseResponse[OrderQueryData]
	err := c.post(ctx, "/dist_app/v1/order/query/", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("api error code: %d, message: %s", resp.Code, resp.Message)
	}
	return &resp.Data, nil
}

// QueryRelations 查询染色归因
func (c *Client) QueryRelations(ctx context.Context, req RelationQueryRequest) (*RelationQueryData, error) {
	if req.DistAppID == 0 {
		req.DistAppID = c.cfg.DefaultDistAppID
	}
	var resp BaseResponse[RelationQueryData]
	err := c.post(ctx, "/dist_app/v1/relation/query/", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("api error code: %d, message: %s", resp.Code, resp.Message)
	}
	return &resp.Data, nil
}
