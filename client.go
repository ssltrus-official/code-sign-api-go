package codesignapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	defaultBaseURL = "https://ssl.face.racent.com"
	nicsrsBaseURL  = "https://ssl.face.nicsrs.com"
)

// Option 配置客户端。
type Option func(*Client)

// Client 封装远程代码签名 API；初始化后可并发调用，使用期间不要修改注入的 Resty 客户端。
type Client struct {
	httpClient *resty.Client
	debug      bool
	timeout    time.Duration
	baseURL    string
	accessKey  string
	secretKey  string
}

// New 创建客户端，默认超时为 60 秒，不自动重试或重定向。
func New(opts ...Option) *Client {
	c := &Client{
		httpClient: resty.New().SetRedirectPolicy(resty.RedirectPolicyFunc(func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse })),
		debug:      defaultDebug,
		timeout:    60 * time.Second,
		baseURL:    defaultBaseURL,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// WithClient 使用调用方的 Resty 客户端，不修改其配置。
// 注入客户端须关闭重试，并避免设置全局认证；重定向、hook 和 transport 行为由调用方负责。
func WithClient(httpClient *resty.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithDebug 控制 Resty 请求调试；不传参数时开启，调试日志可能包含敏感请求和响应内容。
func WithDebug(enabled ...bool) Option {
	return func(c *Client) {
		c.debug = len(enabled) == 0 || enabled[0]
	}
}

// WithTimeout 设置每次调用的超时；零值表示仅使用 context 和 HTTP 客户端的超时。
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithBaseURL 设置服务地址，支持 HTTP 和 HTTPS；生产环境应使用 HTTPS。
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
}

// WithNicSRS 使用 NICSRS 服务地址。
func WithNicSRS() Option {
	return func(c *Client) {
		c.baseURL = nicsrsBaseURL
	}
}

// WithAuthKey 设置 HTTP Basic Auth 凭证。
func WithAuthKey(accessKey, secretKey string) Option {
	return func(c *Client) {
		c.accessKey = strings.TrimSpace(accessKey)
		c.secretKey = strings.TrimSpace(secretKey)
	}
}

// responseEnvelope 保留原始业务数据，并区分缺失状态码与成功码零。
type responseEnvelope struct {
	Code    *int            `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) validate(requireAuth bool) error {
	if c == nil || c.httpClient == nil {
		return fmt.Errorf("codesign: client is nil")
	}
	parsedURL, err := url.Parse(c.baseURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Hostname() == "" {
		return fmt.Errorf("codesign: base url must be an absolute HTTP or HTTPS URL")
	}
	if parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.ForceQuery || parsedURL.Fragment != "" {
		return fmt.Errorf("codesign: base url must not contain credentials, query or fragment")
	}
	if c.timeout < 0 {
		return fmt.Errorf("codesign: timeout must not be negative")
	}
	if c.httpClient.RetryCount != 0 {
		return fmt.Errorf("codesign: resty retries must be disabled to avoid duplicate signing")
	}
	if c.httpClient.UserInfo != nil || c.httpClient.Token != "" || c.httpClient.Header.Get("Authorization") != "" {
		return fmt.Errorf("codesign: use WithAuthKey instead of resty global authentication")
	}
	if requireAuth && (c.accessKey == "" || c.secretKey == "") {
		return fmt.Errorf("codesign: access key or secret key is empty")
	}
	return nil
}

func (c *Client) execute(ctx context.Context, path string, requestBody, responseData any, requireAuth bool) error {
	if err := c.validate(requireAuth); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	request := c.httpClient.R().SetContext(ctx).
		SetHeader("User-Agent", "code-sign-api-go/"+sdkVersion).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetDebug(c.debug).
		SetBody(requestBody)
	if requireAuth {
		request.SetBasicAuth(c.accessKey, c.secretKey)
	}
	response, err := request.Post(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("codesign: POST %s failed: %w", path, err)
	}
	var apiResponse responseEnvelope
	decodeErr := json.Unmarshal(response.Body(), &apiResponse)
	if !response.IsSuccess() {
		apiErr := &APIError{Method: http.MethodPost, Path: path, StatusCode: response.StatusCode()}
		if decodeErr == nil && apiResponse.Code != nil {
			apiErr.Code, apiErr.Message = *apiResponse.Code, apiResponse.Message
		}
		return apiErr
	}
	if decodeErr != nil {
		return fmt.Errorf("codesign: POST %s invalid response JSON: %w", path, decodeErr)
	}
	if apiResponse.Code == nil {
		return fmt.Errorf("codesign: POST %s response code is missing", path)
	}
	if *apiResponse.Code != 0 {
		return &APIError{Method: http.MethodPost, Path: path, StatusCode: response.StatusCode(), Code: *apiResponse.Code, Message: apiResponse.Message}
	}
	if responseData == nil {
		return nil
	}
	if len(apiResponse.Data) == 0 || bytes.Equal(bytes.TrimSpace(apiResponse.Data), []byte("null")) {
		return fmt.Errorf("codesign: POST %s response data is missing", path)
	}
	if err = json.Unmarshal(apiResponse.Data, responseData); err != nil {
		return fmt.Errorf("codesign: POST %s invalid response data: %w", path, err)
	}
	return nil
}
