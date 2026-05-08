package utils

import (
	"context"
	"net"
	"net/http"
	"scutum/cmd/internal/clients"
)

func GetPlatformClient() *clients.DockerClient {
	dial := func(_ context.Context) (net.Conn, error) {
		return net.Dial("unix", "/var/run/docker.sock")
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dial(ctx)
		},
	}
	return clients.NewDockerClient(&http.Client{Transport: transport}, "http://localhost/v1.45", dial)
}
