package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

// KubernetesConfig holds the data needed to talk to the API
type KubernetesConfig struct {
	Host       string
	Token      string
	HTTPClient *http.Client
}

// GetInClusterConfig reads the ServiceAccount files injected by Kubernetes
func GetInClusterConfig() (*KubernetesConfig, error) {
	const (
		tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
		caPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
		host      = "https://kubernetes.default.svc"
	)

	token, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}

	caCert, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return &KubernetesConfig{
		Host:       host,
		Token:      string(token),
		HTTPClient: client,
	}, nil
}
