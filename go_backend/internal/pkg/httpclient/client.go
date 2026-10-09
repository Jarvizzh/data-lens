package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"time"
)

// Config 统一 HTTP 客户端配置
type Config struct {
	Timeout               time.Duration // 请求总超时
	DialTimeout           time.Duration // TCP 建连超时
	KeepAlive             time.Duration // TCP KeepAlive 周期
	TLSHandshakeTimeout   time.Duration // TLS 握手超时 (防跨网握手超时)
	ResponseHeaderTimeout time.Duration // 读取响应头超时
	IdleConnTimeout       time.Duration // 空闲连接保活时长
	MaxIdleConns          int           // 连接池最大空闲连接
	MaxIdleConnsPerHost   int           // 单 Host 最大空闲连接
	MaxRetries            int           // 最大重试次数 (默认 3 次)
	BaseRetryInterval     time.Duration // 基础重试退避间隔 (默认 500ms)
	InsecureSkipVerify    bool          // 是否跳过证书链完整性校验
}

// DefaultConfig 针对跨境及云原生三方 API 优化的默认配置
func DefaultConfig() Config {
	return Config{
		Timeout:               60 * time.Second,
		DialTimeout:           20 * time.Second,
		KeepAlive:             30 * time.Second,
		TLSHandshakeTimeout:   25 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		MaxRetries:            3,
		BaseRetryInterval:     500 * time.Millisecond,
		InsecureSkipVerify:    true,
	}
}

// Option 配置修改选项
type Option func(*Config)

// WithMaxRetries 自定义最大重试次数
func WithMaxRetries(retries int) Option {
	return func(c *Config) {
		if retries > 0 {
			c.MaxRetries = retries
		}
	}
}

// WithTimeout 自定义请求总超时
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		if timeout > 0 {
			c.Timeout = timeout
		}
	}
}

// WithBaseRetryInterval 自定义重试等待基数
func WithBaseRetryInterval(interval time.Duration) Option {
	return func(c *Config) {
		if interval > 0 {
			c.BaseRetryInterval = interval
		}
	}
}

// Client 统一封装的 HTTP 客户端，所有请求默认具备透明自动重试能力
type Client struct {
	inner *http.Client
	cfg   Config
}

// NewClient 创建统一 HTTP 客户端
func NewClient(opts ...Option) *Client {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.DialTimeout,
			KeepAlive: cfg.KeepAlive,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			MinVersion:         tls.VersionTLS12,
		},
	}

	return &Client{
		inner: &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
		},
		cfg: cfg,
	}
}

// Do 发送 HTTP 请求并自动应用重试机制 (默认 3 次)
// 对网络建连超时、TLS 握手超时、网络断开以及服务端 5xx / 429 异常自动执行阶梯退避重试；
// 自动保留并重置 POST/PUT 等带 Body 请求的流状态，上层调用方无需重复编写任何重试循环。
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("http request is nil")
	}

	ctx := req.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// 若存在请求体但未挂载 GetBody 生成器，预先缓存并挂载以支持重试回放
	if req.Body != nil && req.GetBody == nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body for retry buffer failed: %w", err)
		}
		_ = req.Body.Close()
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
		req.Body, _ = req.GetBody()
	}

	maxRetries := c.cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	var lastResp *http.Response

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// 上下文若已取消或超时，直接中断重试
		if ctx.Err() != nil {
			if lastResp != nil {
				_ = lastResp.Body.Close()
			}
			return nil, ctx.Err()
		}

		// 重试时重新生成可读的 Body
		if attempt > 1 && req.GetBody != nil {
			var err error
			req.Body, err = req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("reset request body for attempt %d failed: %w", attempt, err)
			}
		}

		resp, err := c.inner.Do(req)
		if err != nil {
			lastErr = err
			if attempt == maxRetries || ctx.Err() != nil {
				break
			}
			// 带随机抖动 (Jitter) 的阶梯退避等待，杜绝惊群重试
			if err := waitRetry(ctx, attempt, c.cfg.BaseRetryInterval); err != nil {
				return nil, err
			}
			continue
		}

		// 若服务端返回可重试状态码 (5xx 或 429) 且未到最后一次尝试，则清空 Body 准备重试
		if (resp.StatusCode >= 500 || resp.StatusCode == 429) && attempt < maxRetries {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			lastErr = fmt.Errorf("server returned status code: %d", resp.StatusCode)
			if err := waitRetry(ctx, attempt, c.cfg.BaseRetryInterval); err != nil {
				return nil, err
			}
			continue
		}

		// 正常成功 (或最终状态码)，返回响应
		return resp, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("http request failed after %d attempts: %w", maxRetries, lastErr)
	}

	return lastResp, nil
}

// waitRetry 等待重试退避时间，叠加随机抖动并支持 Context 取消中断
func waitRetry(ctx context.Context, attempt int, base time.Duration) error {
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	baseBackoff := time.Duration(attempt) * base
	// 叠加 0 ~ 50% 随机扰动 (Full Jitter)，彻底打破并发重试波峰重叠
	jitter := time.Duration(rand.Int63n(int64(baseBackoff / 2)))
	totalBackoff := baseBackoff + jitter

	timer := time.NewTimer(totalBackoff)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
