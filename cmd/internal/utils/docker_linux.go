package utils

import (
	"context"
	"net"
	"net/http"
	"os"
	"scutum/cmd/internal/clients"
)

const dockerSocketPath = "/var/run/docker.sock"

// IsDockerAvailable reports whether the Docker socket exists and is reachable.
// Use this to return a clean 503 instead of a raw dial error when the socket
// is not mounted (e.g. running inside a Kubernetes pod without docker.enabled).
func IsDockerAvailable() bool {
	_, err := os.Stat(dockerSocketPath)
	return err == nil
}

func GetPlatformClient() *clients.DockerClient {
	dial := func(_ context.Context) (net.Conn, error) {
		return net.Dial("unix", dockerSocketPath)
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dial(ctx)
		},
	}
	return clients.NewDockerClient(&http.Client{Transport: transport}, "http://localhost/v1.45", dial)
}
