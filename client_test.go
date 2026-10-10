package codesignapi_test

import (
	"bufio"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"

	codesignapi "github.com/ssltrus-official/code-sign-api-go"
)

func loadTestConfig(t *testing.T) map[string]string {
	t.Helper()

	file, err := os.Open(".env")
	if errors.Is(err, os.ErrNotExist) {
		t.Skip(".env is not configured")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err = file.Close(); err != nil {
			t.Error(err)
		}
	}()

	config := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found {
			config[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return config
}

func TestClient(t *testing.T) {
	config := loadTestConfig(t)
	if config["BASE_URL"] == "" {
		t.Skip("BASE_URL is not configured")
	}
	client := codesignapi.New(
		codesignapi.WithDebug(false),
		codesignapi.WithBaseURL(config["BASE_URL"]),
		codesignapi.WithAuthKey(config["ACCESS_KEY"], config["SECRET_KEY"]),
	)
	ctx := context.Background()

	t.Run("GetVersion", func(t *testing.T) {
		request := &codesignapi.GetVersionReq{
			Type:     codesignapi.CliTypeCLI,
			Platform: runtime.GOOS + "/" + runtime.GOARCH,
		}
		response, err := client.GetVersion(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if response == nil {
			t.Fatal("version response is nil")
		}
		if response.Latest != (response.Version == "" || response.Version == request.Current) {
			t.Fatal("unexpected latest version state")
		}
	})

	t.Run("Authenticated", func(t *testing.T) {
		if config["ACCESS_KEY"] == "" || config["SECRET_KEY"] == "" {
			t.Skip("ACCESS_KEY or SECRET_KEY is not configured")
		}

		t.Run("Login", func(t *testing.T) {
			response, err := client.Login(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if response == nil || response.Name == "" {
				t.Fatal("user name is missing")
			}
		})

		t.Run("GetCert", func(t *testing.T) {
			if config["CERT_CODE"] == "" {
				t.Skip("CERT_CODE is not configured")
			}
			response, err := client.GetCert(ctx, &codesignapi.GetCertReq{Code: config["CERT_CODE"]})
			if err != nil {
				t.Fatal(err)
			}
			if response == nil {
				t.Fatal("certificate response is nil")
			}
			block, _ := pem.Decode([]byte(response.Pem))
			if block == nil || block.Type != "CERTIFICATE" {
				t.Fatal("certificate PEM is invalid")
			}
			if _, err := x509.ParseCertificate(block.Bytes); err != nil {
				t.Fatal("certificate DER is invalid")
			}
		})

		t.Run("ListCert", func(t *testing.T) {
			response, err := client.ListCert(ctx, &codesignapi.ListCertReq{
				Paging: &codesignapi.Paging{Page: 1, Limit: 10},
			})
			if err != nil {
				t.Fatal(err)
			}
			if response == nil || response.Paging == nil {
				t.Fatal("certificate pagination is missing")
			}
			for _, record := range response.Records {
				if record == nil || record.Code == "" {
					t.Fatal("certificate code is missing")
				}
			}
		})

		t.Run("ListRecord", func(t *testing.T) {
			response, err := client.ListRecord(ctx, &codesignapi.ListRecordReq{
				Paging: &codesignapi.Paging{Page: 1, Limit: 10},
				Code:   config["CERT_CODE"],
			})
			if err != nil {
				t.Fatal(err)
			}
			if response == nil || response.Paging == nil {
				t.Fatal("record pagination is missing")
			}
			for _, record := range response.Records {
				if record == nil || record.Id == "" {
					t.Fatal("signing record id is missing")
				}
				if config["CERT_CODE"] != "" && record.Code != config["CERT_CODE"] {
					t.Fatal("signing record certificate does not match the filter")
				}
			}
		})

		t.Run("Sign", func(t *testing.T) {
			if config["CERT_CODE"] == "" {
				t.Skip("CERT_CODE is not configured")
			}
			certificateResponse, err := client.GetCert(ctx, &codesignapi.GetCertReq{Code: config["CERT_CODE"]})
			if err != nil {
				t.Fatal(err)
			}
			if certificateResponse == nil {
				t.Fatal("certificate response is nil")
			}
			block, _ := pem.Decode([]byte(certificateResponse.Pem))
			if block == nil || block.Type != "CERTIFICATE" {
				t.Fatal("certificate PEM is invalid")
			}
			certificate, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				t.Fatal("certificate DER is invalid")
			}
			publicKey, ok := certificate.PublicKey.(*rsa.PublicKey)
			if !ok {
				t.Fatal("signing certificate public key is not RSA")
			}
			// 每次成功调用都会消耗真实签名额度，不自动重试。
			digest := sha256.Sum256([]byte("code-sign-api-go API test"))
			response, err := client.Sign(ctx, &codesignapi.SignReq{
				Code:      config["CERT_CODE"],
				Digest:    base64.StdEncoding.EncodeToString(digest[:]),
				Algorithm: codesignapi.AlgorithmSHA256,
				Padding:   codesignapi.PaddingPKCS1,
			})
			if err != nil {
				t.Fatal(err)
			}
			if response == nil || response.Id == "" {
				t.Fatal("signing record id is missing")
			}
			signature, err := base64.StdEncoding.DecodeString(response.Signature)
			if err != nil || len(signature) == 0 {
				t.Fatal("signature encoding is invalid")
			}
			if err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
				t.Fatal("signature verification failed")
			}

			t.Run("ReportSign", func(t *testing.T) {
				if err = client.ReportSign(ctx, &codesignapi.ReportSignReq{
					Id:     response.Id,
					Status: codesignapi.ReportSignSuccess,
				}); err != nil {
					t.Fatal(err)
				}
			})
		})
	})
}
