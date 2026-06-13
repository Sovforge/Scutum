package tests

import (
	"os"
	"path/filepath"
	"testing"

	"scutum/cmd/internal/utils"
)

func TestIsDockerAvailable_MissingSocket(t *testing.T) {
	// Point to a path that does not exist — must return false.
	orig := utils.DockerSocketPath
	utils.DockerSocketPath = "/tmp/scutum-nonexistent-docker.sock"
	defer func() { utils.DockerSocketPath = orig }()

	if utils.IsDockerAvailable() {
		t.Error("expected IsDockerAvailable to return false for missing socket")
	}
}

func TestIsDockerAvailable_PresentSocket(t *testing.T) {
	// Create a temporary file to stand in for the socket.
	tmp := filepath.Join(t.TempDir(), "docker.sock")
	f, err := os.Create(tmp)
	if err != nil {
		t.Fatalf("setup: create temp socket file: %v", err)
	}
	f.Close()

	orig := utils.DockerSocketPath
	utils.DockerSocketPath = tmp
	defer func() { utils.DockerSocketPath = orig }()

	if !utils.IsDockerAvailable() {
		t.Error("expected IsDockerAvailable to return true when socket file exists")
	}
}
