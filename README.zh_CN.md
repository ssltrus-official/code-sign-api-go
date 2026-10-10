# code-sign-api-go

[English](README.md) · [简体中文](README.zh_CN.md)

[![CI](https://github.com/ssltrus-official/code-sign-api-go/actions/workflows/golang.yml/badge.svg)](https://github.com/ssltrus-official/code-sign-api-go/actions/workflows/golang.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ssltrus-official/code-sign-api-go.svg)](https://pkg.go.dev/github.com/ssltrus-official/code-sign-api-go)

sslTrus 远程代码签名 Go API 客户端，要求 Go 1.26.0 或更高版本。
直接依赖仅为 [Resty v2](https://github.com/go-resty/resty)，其间接依赖由 Go modules 管理。

## 安装与初始化

```sh
go get github.com/ssltrus-official/code-sign-api-go
```

```go
import (
    "context"
    "os"
    "time"

    codesignapi "github.com/ssltrus-official/code-sign-api-go"
)

client := codesignapi.New(
    codesignapi.WithAuthKey(os.Getenv("ACCESS_KEY"), os.Getenv("ACCESS_SECRET")),
    codesignapi.WithTimeout(60*time.Second),
)
ctx := context.Background()
```

默认地址为 `https://ssl.face.racent.com`。使用 `WithNicSRS()` 切换到
`https://ssl.face.nicsrs.com`，或通过 `WithBaseURL()` 设置交付方提供的地址。
生产环境应使用 HTTPS。

## 接口

所有接口均使用 POST。除 `GetVersion` 外，接口使用 Access Key / Access Secret
进行 HTTP Basic Auth。方法均接收 `context.Context`。
传入 nil context 时使用 `context.Background()`，配置的 SDK 超时仍然生效。

| 方法         | 路径                       | 请求             | 返回                     |
|--------------|----------------------------|------------------|--------------------------|
| `GetVersion` | `/v1/codesign/version`     | `*GetVersionReq` | `*Version, error`        |
| `Login`      | `/v1/codesign/login`       | 无               | `*UserData, error`       |
| `GetCert`    | `/v1/codesign/cert`        | `*GetCertReq`    | `*GetCertResp, error`    |
| `ListCert`   | `/v1/codesign/certs`       | `*ListCertReq`   | `*ListCertResp, error`   |
| `ListRecord` | `/v1/codesign/records`     | `*ListRecordReq` | `*ListRecordResp, error` |
| `Sign`       | `/v1/codesign/sign`        | `*SignReq`       | `*SignData, error`       |
| `ReportSign` | `/v1/codesign/report-sign` | `*ReportSignReq` | `error`                  |

`ListCert` 和 `ListRecord` 的请求可为 nil，分页默认值由服务端决定，不自动遍历所有页。
`Time` 保留原始时间字符串，调用 `.Time()` 解析；空值或无效格式返回 `time.Time{}`。
`Version.Latest` 表示返回版本为空或与请求的 `Current` 相同。

`GetVersionReq.Platform` 必填且仅支持以下常量，其他值会在发送请求前返回错误。
`SignExtra.Platform` 接受任意平台字符串，空值使用当前运行平台，不校验平台范围。
两个字段仍使用 `string` 类型。

| 常量                   | 平台            |
|------------------------|-----------------|
| `PlatformDarwinAMD64`  | `darwin/amd64`  |
| `PlatformDarwinARM64`  | `darwin/arm64`  |
| `PlatformLinuxAMD64`   | `linux/amd64`   |
| `PlatformLinuxARM64`   | `linux/arm64`   |
| `PlatformWindowsAMD64` | `windows/amd64` |

```go
certs, err := client.ListCert(ctx, &codesignapi.ListCertReq{
    Paging: &codesignapi.Paging{Page: 1, Limit: 20},
})
if err != nil {
    return err
}
_ = certs
```

## 原始哈希签名

签名契约见[公开 API 参考](https://help.racent.com/zh/remote-code-signing/integration-guide/codesign-api-reference/)。
SDK 封装原始哈希签名。

调用方须按文件格式和签名工具要求计算哈希，再将哈希原始字节编码为 Base64。
SDK 不计算文件摘要、不读取文件、不附加签名或时间戳。

以下代码假定 `hashBytes` 已由调用方正确计算，并非直接对整个可执行文件计算普通 SHA256：

```go
signed, err := client.Sign(ctx, &codesignapi.SignReq{
    Code:      certCode,
    Digest:    base64.StdEncoding.EncodeToString(hashBytes),
    Algorithm: codesignapi.AlgorithmSHA256,
    Padding:   codesignapi.PaddingPKCS1,
})
if err != nil {
    return err
}
signature, err := base64.StdEncoding.DecodeString(signed.Signature)
if err != nil {
    return err
}
// 调用方完成签名结果的本地处理后，再上报最终状态。
_ = signature
```

此片段需要导入标准库 `encoding/base64`。支持 SHA1、SHA256、SHA384、SHA512，
填充固定为 PKCS1。SDK 校验 Base64 编码及摘要字节长度与算法一致。
签名返回 `Id` 和 Base64 编码的 `Signature`。

`Sign` 会将 `SignExtra.Platform`、`Version`、`Hostname` 的空值分别补齐为当前运行平台、
SDK 版本和当前主机名，`Extra` 为 nil 时也会补齐。主机名获取失败时省略该字段。
调用方已提供的值会保留，原始请求和 `Extra` 均不会被修改。
`Revision`、`Time` 和文件信息仍由调用方提供，SDK 不读取文件。
文件信息使用 `signing_filename/signing_filesize` 请求字段，查询记录使用
`signingFilename/signingFilesize` 响应字段。

成功签名会消耗签名额度；即使网络超时或响应解析失败，服务端也可能已完成签名。
客户端不自动重试，调用方应确认结果后再决定是否重试。

```go
err = client.ReportSign(ctx, &codesignapi.ReportSignReq{
    Id:     signed.Id,
    Status: codesignapi.ReportSignSuccess, // 本地处理失败时使用 ReportSignFailed
})
```

上报仅记录客户端处理状态，不影响签名计次；SDK 不自动上报。

## 错误与客户端配置

HTTP 非 2xx 或业务 `code != 0` 返回 `*APIError`，不依赖错误文案判断业务分支：

```go
var apiErr *codesignapi.APIError
if errors.As(err, &apiErr) {
    // 根据 apiErr.StatusCode 和 apiErr.Code 处理错误。
}
```

此片段需要导入标准库 `errors`。网络、取消和解析错误保留错误链；
空响应、缺失业务码或缺失业务数据不会作为成功返回。
`ReportSign` 允许成功响应的 `data` 为 null。

- 默认每次调用最多等待 60 秒；context 或 HTTP 客户端设置的更短超时也生效。
- `WithTimeout(0)` 关闭 SDK 的超时，保留 context 和 HTTP 客户端的超时。
- 默认客户端不自动重定向，避免重复发送签名请求。
- `WithClient(*resty.Client)` 注入自定义客户端，SDK 不修改其配置。
  注入客户端必须关闭 Resty 重试，不应配置全局认证；通过 `WithAuthKey` 设置凭证。
  调用方负责注入客户端的重定向策略、hook、transport 和日志行为，并避免使用期间修改配置。
- `WithDebug()` 或 `WithDebug(true)` 开启 Resty 请求调试，SDK 不单独记录请求日志。
  Resty 调试日志可能包含敏感请求头、请求体及响应体，`WithDebug(false)` 关闭 SDK 请求的调试日志。
  环境变量按 `SSLTRUS_CODESIGN_DEBUG`、`SSLTRUS_DEBUG`、`DEBUG`
  的顺序读取第一个已设置值，`1/t/true/yes/y/on` 开启日志，其余值关闭日志。

生产环境应关闭调试，并限制包含凭证、摘要或签名内容的调试日志的访问权限。

## 开发

Makefile 面向 Linux、macOS 和 Windows WSL 的 POSIX Shell 环境。
需要 Go 1.26.0+、make、gofmt；静态检查需要 golangci-lint v2.14.0。
工具独立安装，不加入 SDK 的 go.mod：

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
# 可选；未安装时 make test 回退到 go test。
go install gotest.tools/gotestsum@latest
```

工具安装可能需要比 SDK 更高版本的 Go 工具链；Go 默认可自动下载所需工具链。

| 命令                | 行为                                                  |
|---------------------|-------------------------------------------------------|
| `make help`         | 显示帮助，也是 make 的默认目标                        |
| `make tidy`         | 显式整理 go.mod 和 go.sum                             |
| `make format`       | 使用 gofmt 格式化源码                                 |
| `make format-check` | 检查格式，不修改文件                                  |
| `make vet`          | 执行 go vet                                           |
| `make lint`         | 使用根 .golangci.yml 检查                             |
| `make compile`      | 编译全部包，不运行代码                                |
| `make test`         | 执行测试，优先使用 gotestsum，启用 race 检查          |
| `make check`        | 依次执行 format-check、vet、lint、compile，不运行测试 |

```sh
make check
make compile GOOS=windows GOARCH=amd64
make test
```

可覆盖变量：`GO`、`GOFMT`、`GOLANGCI_LINT`、`GOOS`、`GOARCH`。
检查和编译使用只读依赖模式，不隐式执行 tidy。format 处理根包的 Go 文件。

`client_test.go` 直接测试真实服务的全部 7 个接口，不使用 mock 或 build tag 隔离。
从 `.env` 读取以下配置：

| 配置                       | 使用范围                                            |
|----------------------------|-----------------------------------------------------|
| `BASE_URL`                 | 全部测试                                            |
| `ACCESS_KEY`、`SECRET_KEY` | 全部需要认证的测试                                  |
| `CERT_CODE`                | GetCert 和 Sign 必填，ListRecord 的可选证书筛选条件 |

文件使用普通 `KEY=value` 行，忽略空行和以 `#` 开头的注释行。
缺少配置时跳过对应测试；读取文件或调用接口失败时测试失败。
测试关闭 debug，不输出完整响应、摘要或签名。

Sign 测试先获取对应证书，再对固定测试数据计算 SHA256 并调用真实签名接口。
每次成功调用都会消耗签名额度。使用证书公钥验证 RSA PKCS#1 签名通过后，
嵌套的 ReportSign 测试才使用返回的记录 ID 上报成功。
测试不自动重试；配置齐全时执行 `make test` 会包含这次真实签名调用。

仅查询版本接口可以使用：

```sh
go test -mod=readonly -count=1 -run '^TestClient$/^GetVersion$' ./...
```

`.env` 不纳入 Git，示例和文档不保存真实凭证。跳过测试不代表真实接口已验证。

GitHub Actions 对 PR、分支 push 和 `v*` tag 执行依赖一致性、格式、vet、lint、编译和测试检查。
CI 不配置 `.env` 或服务凭证。推送 `v*` 标签且检查通过后，自动创建 GitHub Release 并生成发布说明，
随后通过 `proxy.golang.org` 和 `sum.golang.org` 下载标签对应的模块版本，预热 Go 模块服务。
Dependabot 每周检查 Go modules 和 GitHub Actions。
