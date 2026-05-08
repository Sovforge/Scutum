package kms

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AWSProvider struct {
	region    string
	keyID     string
	accessKey string
	secretKey string
	client    *http.Client
}

func NewAWSProvider(region, keyID, accessKey, secretKey string) (*AWSProvider, error) {
	return NewAWSProviderWithClient(region, keyID, accessKey, secretKey, nil)
}

func NewAWSProviderWithClient(region, keyID, accessKey, secretKey string, client *http.Client) (*AWSProvider, error) {
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("aws: access_key and secret_key are required")
	}
	if client == nil {
		client = &http.Client{}
	}
	return &AWSProvider{
		region:    region,
		keyID:     keyID,
		accessKey: accessKey,
		secretKey: secretKey,
		client:    client,
	}, nil
}

func (a *AWSProvider) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	body, _ := json.Marshal(map[string]string{
		"KeyId":     a.keyID,
		"Plaintext": base64.StdEncoding.EncodeToString(plaintext),
	})

	resp, err := a.kmsRequest(ctx, "TrentService.Encrypt", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		CiphertextBlob string `json:"CiphertextBlob"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("aws encrypt parse: %w", err)
	}
	return base64.StdEncoding.DecodeString(result.CiphertextBlob)
}

func (a *AWSProvider) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	body, _ := json.Marshal(map[string]string{
		"CiphertextBlob": base64.StdEncoding.EncodeToString(ciphertext),
	})

	resp, err := a.kmsRequest(ctx, "TrentService.Decrypt", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Plaintext string `json:"Plaintext"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("aws decrypt parse: %w", err)
	}
	return base64.StdEncoding.DecodeString(result.Plaintext)
}

func (a *AWSProvider) Name() string { return "aws" }

// Wipe is a no-op for AWS — key material lives in AWS KMS, not in memory.
func (a *AWSProvider) Wipe() {}

// LoadMasterKey is not supported for AWS KMS. Emergency recovery requires
// switching to the local provider first.
func (a *AWSProvider) LoadMasterKey(_ []byte) error {
	return errors.New("aws: LoadMasterKey not supported — emergency recovery requires the local provider")
}

// MasterKeyDigest is not supported for AWS KMS. The master key never exists
// in memory so no digest can be computed.
func (a *AWSProvider) MasterKeyDigest() ([]byte, error) {
	return nil, errors.New("aws: MasterKeyDigest not supported — emergency recovery requires the local provider")
}

func (a *AWSProvider) kmsRequest(ctx context.Context, action string, body []byte) ([]byte, error) {
	endpoint := fmt.Sprintf("https://kms.%s.amazonaws.com/", a.region)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("aws request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", action)

	a.signRequest(req, body)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aws kms call: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("aws read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aws kms error %d: %s", resp.StatusCode, data)
	}
	return data, nil
}

func (a *AWSProvider) signRequest(req *http.Request, body []byte) {
	t := time.Now().UTC()
	date := t.Format("20060102")
	timestamp := t.Format("20060102T150405Z")

	payloadHash := fmt.Sprintf("%x", sha256.Sum256(body))
	req.Header.Set("X-Amz-Date", timestamp)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\nx-amz-target:%s\n",
		req.Header.Get("Content-Type"), req.URL.Host, payloadHash, timestamp, req.Header.Get("X-Amz-Target"))
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date;x-amz-target"

	canonicalRequest := strings.Join([]string{
		req.Method, req.URL.Path, "",
		canonicalHeaders, signedHeaders, payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/kms/aws4_request", date, a.region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%x",
		timestamp, credentialScope, sha256.Sum256([]byte(canonicalRequest)))

	kDate := awsHMAC([]byte("AWS4"+a.secretKey), date)
	kRegion := awsHMAC(kDate, a.region)
	kService := awsHMAC(kRegion, "kms")
	kSigning := awsHMAC(kService, "aws4_request")
	signature := hex.EncodeToString(awsHMAC(kSigning, stringToSign))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		a.accessKey, credentialScope, signedHeaders, signature,
	))
}

func awsHMAC(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
