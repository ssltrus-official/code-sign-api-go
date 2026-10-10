# code-sign-api-go

[English](README.md) · [简体中文](README.zh_CN.md)

[![CI](https://github.com/ssltrus-official/code-sign-api-go/actions/workflows/golang.yml/badge.svg)](https://github.com/ssltrus-official/code-sign-api-go/actions/workflows/golang.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ssltrus-official/code-sign-api-go.svg)](https://pkg.go.dev/github.com/ssltrus-official/code-sign-api-go)

Go API client for sslTrus remote code signing. Requires Go 1.26.0 or later.
The only direct dependency is [Resty v2](https://github.com/go-resty/resty); Go modules manage its transitive dependencies.

## Installation and initialization

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

The default endpoint is `https://ssl.face.racent.com`. Use `WithNicSRS()` for
`https://ssl.face.nicsrs.com`, or `WithBaseURL()` for an endpoint supplied by your service provider.
Use HTTPS in production.

## API methods

All endpoints use POST. Except for `GetVersion`, methods authenticate with
Access Key / Access Secret using HTTP Basic Auth. Every method accepts `context.Context`.
A nil context falls back to `context.Background()`; the configured SDK timeout still applies.

| Method       | Endpoint                   | Request          | Return                   |
|--------------|----------------------------|------------------|--------------------------|
| `GetVersion` | `/v1/codesign/version`     | `*GetVersionReq` | `*Version, error`        |
| `Login`      | `/v1/codesign/login`       | None             | `*UserData, error`       |
| `GetCert`    | `/v1/codesign/cert`        | `*GetCertReq`    | `*GetCertResp, error`    |
| `ListCert`   | `/v1/codesign/certs`       | `*ListCertReq`   | `*ListCertResp, error`   |
| `ListRecord` | `/v1/codesign/records`     | `*ListRecordReq` | `*ListRecordResp, error` |
| `Sign`       | `/v1/codesign/sign`        | `*SignReq`       | `*SignData, error`       |
| `ReportSign` | `/v1/codesign/report-sign` | `*ReportSignReq` | `error`                  |

`ListCert` and `ListRecord` accept nil requests to use server pagination defaults.
They return one page and do not automatically fetch subsequent pages.
`Time` preserves the original timestamp string. Its `.Time()` method returns a zero `time.Time` for empty or invalid values.
`Version.Latest` is true when the returned version is empty or matches the request's `Current`.

`GetVersionReq.Platform` is required and supports only the following constants; other values
return an error before sending the request. `SignExtra.Platform` accepts any platform string
and defaults to the current runtime platform when empty. Both fields retain their `string` type.

| Constant               | Platform        |
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

## Raw hash signing

See the [public API reference](https://help.racent.com/zh/remote-code-signing/integration-guide/codesign-api-reference/) for the signing contract.
The SDK exposes raw hash signing.

Compute the hash according to the file format and signing tool requirements, then Base64-encode the raw hash bytes.
The SDK does not hash or read files, attach signatures, or request timestamps.

The following assumes that `hashBytes` has already been computed correctly. For executable signing, this is not simply a SHA256 hash of the entire file:

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
// Report the final status after processing the signature locally.
_ = signature
```

This snippet requires the standard library's `encoding/base64` package.
Supported algorithms are SHA1, SHA256, SHA384 and SHA512; padding must be PKCS1.
The SDK checks the Base64 encoding and that the decoded hash length matches the selected algorithm.
The response contains `Id` and a Base64-encoded `Signature`.

`Sign` fills empty `SignExtra.Platform`, `Version` and `Hostname` with the current runtime
platform, SDK version and hostname, respectively, including when `Extra` is nil.
If the hostname cannot be obtained, it is omitted. Caller-provided values are preserved,
and the original request and `Extra` are not modified. `Revision`, `Time` and file information
remain caller-provided; the SDK does not read files.
Signing requests use `signing_filename/signing_filesize`; record queries return `signingFilename/signingFilesize`.

A successful signing request consumes signing quota. The server may have completed signing even if the client times out or cannot parse the response.
The client does not retry automatically. Confirm the outcome before retrying.

```go
err = client.ReportSign(ctx, &codesignapi.ReportSignReq{
    Id:     signed.Id,
    Status: codesignapi.ReportSignSuccess, // Use ReportSignFailed if local processing fails.
})
```

Reporting records the final client status without changing signing usage. The SDK does not report automatically.

## Errors and client configuration

HTTP non-2xx responses or nonzero business codes return `*APIError`.
Use status and business codes rather than matching error messages:

```go
var apiErr *codesignapi.APIError
if errors.As(err, &apiErr) {
    // Handle apiErr.StatusCode and apiErr.Code.
}
```

This snippet requires the standard library's `errors` package.
Network, cancellation and decoding errors preserve their error chains.
Empty responses, missing business codes and missing business data are not treated as success.
A successful `ReportSign` response may contain `data: null`.

- Each call has a default timeout of 60 seconds. A shorter context or HTTP client timeout also applies.
- `WithTimeout(0)` disables the SDK timeout while retaining context and HTTP client timeouts.
- The default client does not follow redirects, preventing automatic resubmission of signing requests.
- `WithClient(*resty.Client)` injects a client without changing its configuration.
  Disable Resty retries and avoid global authentication; use `WithAuthKey` for credentials.
  The caller is responsible for redirect policies, hooks, transports and logging, and must not modify the client while it is in use.
- `WithDebug()` or `WithDebug(true)` enables Resty request debugging; the SDK does not write separate request logs.
  Resty debug logs may include sensitive headers and request or response bodies. `WithDebug(false)` disables it for SDK requests.
  Environment variables are checked in order:
  `SSLTRUS_CODESIGN_DEBUG`, `SSLTRUS_DEBUG`, `DEBUG`. The first set variable takes precedence;
  `1/t/true/yes/y/on` enables logging, and other values disable it.

Keep debugging disabled in production and restrict access to debug logs containing credentials, hashes or signatures.

## Development

The Makefile targets Linux, macOS and Windows WSL with a POSIX shell.
It requires Go 1.26.0+, make and gofmt. Linting requires golangci-lint v2.14.0.
Install development tools separately; they are not dependencies in the SDK's go.mod:

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
# Optional; make test falls back to go test when unavailable.
go install gotest.tools/gotestsum@latest
```

Tool installation may require a newer Go toolchain than the SDK; Go can download the required toolchain automatically by default.

| Command             | Behavior                                                   |
|---------------------|------------------------------------------------------------|
| `make help`         | Show help; the default make target                         |
| `make tidy`         | Explicitly tidy go.mod and go.sum                          |
| `make format`       | Format source with gofmt                                   |
| `make format-check` | Check formatting without changing files                    |
| `make vet`          | Run go vet                                                 |
| `make lint`         | Lint using the root .golangci.yml                          |
| `make compile`      | Compile all packages without running code                  |
| `make test`         | Run tests with race detection, preferring gotestsum        |
| `make check`        | Run format-check, vet, lint and compile in order; no tests |

```sh
make check
make compile GOOS=windows GOARCH=amd64
make test
```

Override `GO`, `GOFMT`, `GOLANGCI_LINT`, `GOOS` or `GOARCH` as needed.
Checks and compilation use read-only dependency mode and do not implicitly run tidy.
The format target processes Go files in the root package.

`client_test.go` tests all seven endpoints against the real service, without mocks or build tags.
It reads these settings from `.env`:

| Setting                    | Required by                                                  |
|----------------------------|--------------------------------------------------------------|
| `BASE_URL`                 | All tests                                                    |
| `ACCESS_KEY`, `SECRET_KEY` | All authenticated tests                                      |
| `CERT_CODE`                | GetCert and Sign; optional certificate filter for ListRecord |

The file uses plain `KEY=value` lines; blank lines and lines starting with `#` are ignored.
Missing settings skip the affected tests. File read errors and API errors fail the tests.
Tests disable debug logging and do not print complete responses, hashes or signatures.

The Sign test first fetches the corresponding certificate, then hashes fixed test data with
SHA256 and calls the real signing API. A successful call consumes signing quota.
It verifies the RSA PKCS#1 signature with the certificate's public key before its nested
ReportSign test reports success using the returned record ID.
There is no automatic retry. Running `make test` with complete settings includes this signing call.

To query only the version endpoint:

```sh
go test -mod=readonly -count=1 -run '^TestClient$/^GetVersion$' ./...
```

Do not commit `.env` or put real credentials in examples or documentation. A skipped test does not verify the real API.

GitHub Actions checks dependency consistency, formatting, vet, lint, compilation and tests on PRs, branch pushes and `v*` tags.
CI does not configure `.env` or service credentials. After checks pass for a pushed `v*` tag,
it creates a GitHub Release with generated notes and downloads the tagged module through
`proxy.golang.org` and `sum.golang.org` to warm Go module services.
Dependabot checks Go modules and GitHub Actions weekly.

## License

Copyright 2026 sslTrus.

The Go API client source code and documentation are licensed under the
[Apache License 2.0](LICENSE).
