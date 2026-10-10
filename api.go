package codesignapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"runtime"
)

const (
	CliTypeCLI = 1
	CliTypeGUI = 2

	PlatformDarwinAMD64  = "darwin/amd64"
	PlatformDarwinARM64  = "darwin/arm64"
	PlatformLinuxAMD64   = "linux/amd64"
	PlatformLinuxARM64   = "linux/arm64"
	PlatformWindowsAMD64 = "windows/amd64"

	AlgorithmSHA1   = "SHA1"
	AlgorithmSHA256 = "SHA256"
	AlgorithmSHA384 = "SHA384"
	AlgorithmSHA512 = "SHA512"
	PaddingPKCS1    = "PKCS1"

	ReportSignSuccess = 1
	ReportSignFailed  = 2
)

type GetVersionReq struct {
	// Type 客户端类型：CliTypeCLI 或 CliTypeGUI。
	Type int `json:"type"`
	// Platform 客户端平台，必填，使用预置的 Platform 常量。
	Platform string `json:"platform"`
	// Current 当前客户端版本，用于查询更新并计算 Latest。
	Current string `json:"current"`
}

type Version struct {
	// Latest 客户端计算的更新状态；远端版本为空或等于 Current 时为 true。
	Latest bool `json:"latest,omitempty"`
	// Version 服务端返回的客户端版本。
	Version string `json:"version"`
	// Url 版本安装包的下载地址。
	Url string `json:"url"`
	// Size 版本安装包的大小，单位为字节。
	Size int `json:"size"`
	// Sha256 版本安装包的 SHA256 校验值。
	Sha256 string `json:"sha256"`
	// Note 版本发布说明。
	Note string `json:"note"`
	// Force 服务端是否要求强制更新。
	Force bool `json:"force"`
}

// GetVersion 查询客户端版本，无需认证；Latest 根据请求的 Current 计算。
func (c *Client) GetVersion(ctx context.Context, request *GetVersionReq) (*Version, error) {
	if request == nil {
		return nil, fmt.Errorf("codesign: version request is nil")
	}
	if err := validatePlatform(request.Platform); err != nil {
		return nil, err
	}
	var response Version
	if err := c.execute(ctx, "/v1/codesign/version", request, &response, false); err != nil {
		return nil, err
	}
	response.Latest = response.Version == "" || response.Version == request.Current
	return &response, nil
}

type UserData struct {
	// Name 访问凭证对应的用户名称。
	Name string `json:"name"`
}

// Login 校验访问凭证，不保存会话或修改客户端配置。
func (c *Client) Login(ctx context.Context) (*UserData, error) {
	var response UserData
	if err := c.execute(ctx, "/v1/codesign/login", nil, &response, true); err != nil {
		return nil, err
	}
	return &response, nil
}

type GetCertReq struct {
	// Code 要获取的证书编号。
	Code string `json:"code,omitempty"`
}

type GetCertResp struct {
	// Pem PEM 编码的签名证书，不包含私钥。
	Pem string `json:"pem,omitempty"`
}

// GetCert 获取证书 PEM，不包含私钥。
func (c *Client) GetCert(ctx context.Context, request *GetCertReq) (*GetCertResp, error) {
	if request == nil {
		return nil, fmt.Errorf("codesign: certificate request is nil")
	}
	var response GetCertResp
	if err := c.execute(ctx, "/v1/codesign/cert", request, &response, true); err != nil {
		return nil, err
	}
	return &response, nil
}

// Paging 保留服务端分页字段；Page 和 Limit 的默认值由服务端决定。
type Paging struct {
	// Page 请求页码或服务端返回的当前页码，默认值由服务端决定。
	Page int `json:"page,omitempty"`
	// Limit 请求每页数量或服务端返回的每页数量，默认值由服务端决定。
	Limit int `json:"limit,omitempty"`
	// Total 服务端返回的记录总数。
	Total int `json:"total,omitempty"`
	// Pages 服务端返回的总页数。
	Pages int `json:"pages,omitempty"`
}

type ListCertReq struct {
	// Paging 证书列表的分页参数。
	*Paging
}

type ListCertResp struct {
	// Paging 服务端返回的证书列表分页信息。
	*Paging

	// Records 当前页的证书记录。
	Records []*CertData `json:"records,omitempty"`
}

type CertData struct {
	// Code 证书编号，用于获取证书和发起签名。
	Code string `json:"code"`
	// ProductName 证书产品名称。
	ProductName string `json:"productName"`
	// CertCommonName 证书主体的通用名称。
	CertCommonName string `json:"certCommonName"`
	// CertSubject 证书主体信息。
	CertSubject string `json:"certSubject"`
	// QuotaTotal 签名总额度。
	QuotaTotal int `json:"quotaTotal"`
	// QuotaUsed 已使用的签名额度。
	QuotaUsed int `json:"quotaUsed"`
	// StartAt 服务端返回的生效时间，使用 RFC3339 时间字符串。
	StartAt Time `json:"startAt"`
	// EndAt 服务端返回的到期时间，使用 RFC3339 时间字符串。
	EndAt Time `json:"endAt"`
}

// ListCert 查询一页证书；nil 请求使用服务端默认分页。
func (c *Client) ListCert(ctx context.Context, request *ListCertReq) (*ListCertResp, error) {
	if request == nil {
		request = &ListCertReq{}
	}
	var response ListCertResp
	if err := c.execute(ctx, "/v1/codesign/certs", request, &response, true); err != nil {
		return nil, err
	}
	return &response, nil
}

type ListRecordReq struct {
	// Paging 签名记录的分页参数。
	*Paging

	// Code 按证书编号筛选，空值时省略该条件。
	Code string `json:"code,omitempty"`
	// StartAt 筛选时间范围的起始时间，使用 RFC3339 时间字符串。
	StartAt Time `json:"startAt,omitempty"`
	// EndAt 筛选时间范围的结束时间，使用 RFC3339 时间字符串。
	EndAt Time `json:"endAt,omitempty"`
}

type ListRecordResp struct {
	// Paging 服务端返回的签名记录分页信息。
	*Paging

	// Records 当前页的签名记录。
	Records []*RecordData `json:"records,omitempty"`
}

type RecordData struct {
	// Id 签名记录 ID。
	Id string `json:"id"`
	// Code 执行签名的证书编号。
	Code string `json:"code"`
	// Method 服务端记录的签名算法或方式标识。
	Method int `json:"method"`
	// Status 服务端签名状态：0 处理中，1 成功，2 失败。
	Status int `json:"status"`
	// Took 服务端记录的签名处理耗时。
	Took int `json:"took"`
	// ClientStatus 客户端处理状态：0 未上报或处理中，1 成功，2 失败。
	ClientStatus int `json:"clientStatus"`
	// Extra 服务端记录的客户端和请求来源信息。
	Extra *RecordExtra `json:"extra"`
	// CreatedAt 签名记录创建时间，使用 RFC3339 时间字符串。
	CreatedAt Time `json:"createdAt"`
}

// RecordExtra 使用查询接口的 camelCase 字段，与 SignExtra 的请求字段不同。
type RecordExtra struct {
	// Platform 发起签名的客户端平台。
	Platform string `json:"platform"`
	// Version 发起签名的客户端版本。
	Version string `json:"version"`
	// RemoteIp 服务端记录的请求来源 IP 地址。
	RemoteIp string `json:"remoteIp"`
	// Hostname 调用方提供的主机名。
	Hostname string `json:"hostname"`
	// SigningFilename 调用方提供的签名文件名或路径，未提供时为空。
	SigningFilename string `json:"signingFilename"`
	// SigningFilesize 调用方提供的签名文件大小，单位为字节；零值也可能表示未提供。
	SigningFilesize int `json:"signingFilesize"`
}

// ListRecord 查询一页签名记录；nil 请求使用服务端默认分页。
func (c *Client) ListRecord(ctx context.Context, request *ListRecordReq) (*ListRecordResp, error) {
	if request == nil {
		request = &ListRecordReq{}
	}
	var response ListRecordResp
	if err := c.execute(ctx, "/v1/codesign/records", request, &response, true); err != nil {
		return nil, err
	}
	return &response, nil
}

// SignReq 提交哈希原始字节的 Base64 编码；Digest 不是十六进制摘要或完整文件内容。
type SignReq struct {
	// Code 必填，选择远程签名证书的证书编号。
	Code string `json:"code"`
	// Digest 必填，哈希原始字节的 Base64 编码，字节长度须与 Algorithm 一致。
	Digest string `json:"digest"`
	// Algorithm 必填，支持 SHA1、SHA256、SHA384 和 SHA512。
	Algorithm string `json:"algorithm"`
	// Padding 必填，RSA 填充模式，当前固定为 PKCS1。
	Padding string `json:"padding"`
	// Extra 可选，审计和诊断信息；SDK 补齐平台、版本和主机名的空值。
	Extra *SignExtra `json:"extra,omitempty"`
}

// SignExtra 保留调用方提供的值；SDK 仅补齐平台、版本和主机名，不采集文件信息。
type SignExtra struct {
	// Platform 客户端平台，空值使用当前运行平台；调用方可指定任意平台标识。
	Platform string `json:"platform,omitempty"`
	// Version 客户端版本，空值使用 SDK 版本。
	Version string `json:"version,omitempty"`
	// Revision 客户端构建 revision。
	Revision string `json:"revision,omitempty"`
	// Time 客户端构建时间或请求时间。
	Time string `json:"time,omitempty"`
	// Hostname 发起签名的主机名，空值尝试获取当前主机名，获取失败时省略。
	Hostname string `json:"hostname,omitempty"`
	// SigningFilename 签名文件名或路径，由调用方按需要脱敏。
	SigningFilename string `json:"signing_filename,omitempty"`
	// SigningFilesize 签名文件大小，单位为字节；零值时省略该字段。
	SigningFilesize int64 `json:"signing_filesize,omitempty"`
}

type SignData struct {
	// Id 签名记录 ID，用于 ReportSign 上报客户端处理结果。
	Id string `json:"id"`
	// Signature RSA PKCS#1 签名结果的 Base64 编码。
	Signature string `json:"signature"`
}

// Sign 获取 RSA PKCS#1 签名的 Base64 编码；成功会消耗额度，网络失败时不要盲目重试。
func (c *Client) Sign(ctx context.Context, request *SignReq) (*SignData, error) {
	if request == nil || request.Code == "" {
		return nil, fmt.Errorf("codesign: signing request and certificate code are required")
	}
	digestSize := 0
	switch request.Algorithm {
	case AlgorithmSHA1:
		digestSize = 20
	case AlgorithmSHA256:
		digestSize = 32
	case AlgorithmSHA384:
		digestSize = 48
	case AlgorithmSHA512:
		digestSize = 64
	default:
		return nil, fmt.Errorf("codesign: algorithm must be SHA1, SHA256, SHA384 or SHA512")
	}
	if request.Padding != PaddingPKCS1 {
		return nil, fmt.Errorf("codesign: padding must be PKCS1")
	}
	digest, err := base64.StdEncoding.DecodeString(request.Digest)
	if err != nil || len(digest) != digestSize {
		return nil, fmt.Errorf("codesign: digest must be a Base64 encoded hash matching algorithm")
	}
	requestBody := *request
	extra := SignExtra{}
	if request.Extra != nil {
		extra = *request.Extra
	}
	if extra.Platform == "" {
		extra.Platform = runtime.GOOS + "/" + runtime.GOARCH
	}
	if extra.Version == "" {
		extra.Version = sdkVersion
	}
	if extra.Hostname == "" {
		if hostname, hostnameErr := os.Hostname(); hostnameErr == nil {
			extra.Hostname = hostname
		}
	}
	requestBody.Extra = &extra
	var response SignData
	if err = c.execute(ctx, "/v1/codesign/sign", &requestBody, &response, true); err != nil {
		return nil, err
	}
	if response.Id == "" || response.Signature == "" {
		return nil, fmt.Errorf("codesign: signing response id or signature is missing")
	}
	return &response, nil
}

type ReportSignReq struct {
	// Id 必填，Sign 返回的签名记录 ID。
	Id string `json:"id"`
	// Status 必填，客户端最终处理状态：ReportSignSuccess 为成功，ReportSignFailed 为失败。
	Status int `json:"status"`
}

// ReportSign 上报客户端最终处理结果，不改变远程签名计次。
func (c *Client) ReportSign(ctx context.Context, request *ReportSignReq) error {
	if request == nil || request.Id == "" {
		return fmt.Errorf("codesign: report request and signing record id are required")
	}
	if request.Status != ReportSignSuccess && request.Status != ReportSignFailed {
		return fmt.Errorf("codesign: report status must be 1 or 2")
	}
	return c.execute(ctx, "/v1/codesign/report-sign", request, nil, true)
}

func validatePlatform(platform string) error {
	switch platform {
	case PlatformDarwinAMD64, PlatformDarwinARM64, PlatformLinuxAMD64, PlatformLinuxARM64, PlatformWindowsAMD64:
		return nil
	default:
		return fmt.Errorf("codesign: platform must be darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 or windows/amd64")
	}
}
