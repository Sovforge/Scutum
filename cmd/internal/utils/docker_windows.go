package utils

import (
	"context"
	"net"
	"net/http"
	"scutum/cmd/internal/clients"
)

func GetPlatformClient() *clients.DockerClient {
	// Docker Desktop on Windows exposes the daemon on TCP.
	// Enable "Expose daemon on tcp://localhost:2375" in Docker Desktop settings.
	dial := func(_ context.Context) (net.Conn, error) {
		return net.Dial("tcp", "localhost:2375")
	}
	return clients.NewDockerClient(&http.Client{}, "http://localhost:2375/v1.45", dial)
}
