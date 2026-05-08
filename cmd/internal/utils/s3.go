package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

// S3Config holds the connection details for any provider
type S3Config struct {
	Endpoint  string // e.g., "s3.amazonaws.com" or "192.168.1.50:9000"
	Region    string // e.g., "us-east-1" or "main"
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// SignS3Request applies AWS SigV4 headers.
// It works for any provider that implements the S3 API.
func SignS3Request(req *http.Request, body []byte, cfg S3Config) {
	t := time.Now().UTC()
	date := t.Format("20060102")
	timestamp := t.Format("20060102T150405Z")

	payloadHash := fmt.Sprintf("%x", sha256.Sum256(body))
	req.Header.Set("x-amz-date", timestamp)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	// Canonical Request
	// Note: the Host header exactly as it will be sent
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.Host, payloadHash, timestamp)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method, req.URL.Path, req.URL.RawQuery, canonicalHeaders, signedHeaders, payloadHash)

	// String to Sign
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", date, cfg.Region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%x",
		timestamp, credentialScope, sha256.Sum256([]byte(canonicalRequest)))

	// Signing Key
	kDate := hmacSHA256([]byte("AWS4"+cfg.SecretKey), date)
	kRegion := hmacSHA256(kDate, cfg.Region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")

	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		cfg.AccessKey, credentialScope, signedHeaders, signature)

	req.Header.Set("Authorization", authHeader)
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
