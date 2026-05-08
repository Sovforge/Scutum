//go:build embed_wireguard
// +build embed_wireguard

package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type CommandRunner interface {
	Run(name string, args ...string) error
	Output(name string, args ...string) ([]byte, error)
	CombinedOutput(name string, args ...string) ([]byte, error)
	StdinPipe(name string, args ...string) (PipeWriter, error)
}

type PipeWriter interface {
	Write([]byte) error
	Close() error
}

type realCommandRunner struct{}

func (r *realCommandRunner) Run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func (r *realCommandRunner) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func (r *realCommandRunner) CombinedOutput(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func (r *realCommandRunner) StdinPipe(name string, args ...string) (PipeWriter, error) {
	cmd := exec.Command(name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	cmd.Start()
	return &pipeWriter{stdin: stdin, cmd: cmd}, nil
}

type pipeWriter struct {
	stdin io.WriteCloser
	cmd   *exec.Cmd
}

func (p *pipeWriter) Write(data []byte) error {
	_, err := p.stdin.Write(data)
	return err
}

func (p *pipeWriter) Close() error {
	p.stdin.Close()
	return p.cmd.Wait()
}

var DefaultCommandRunner CommandRunner = &realCommandRunner{}

func SetCommandRunner(runner CommandRunner) {
	DefaultCommandRunner = runner
}

func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("cryptographic failure: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

type InterfaceConfig struct {
	Name       string
	PrivateKey string
	Address    string
	Port       int
	MTU        int
}

type SetupResult struct {
	PublicKey  string
	Address    string
	ListenPort int
}

func SetupInterface(cfg InterfaceConfig) (*SetupResult, error) {
	runner := DefaultCommandRunner
	name := cfg.Name

	runner.Run("ip", "link", "delete", name)

	// Use embedded wg binary for configuration if available, but interface
	// creation still requires the kernel module or wireguard-go.
	if len(EmbeddedWgBinary) > 0 {
		if wgPath, err := initEmbeddedWg(); err == nil {
			log.Println("WireGuard: using embedded wg binary")
			runner = &embeddedRunner{wgPath: wgPath}
		}
	}

	if err := ensureWireGuardInterface(runner, name); err != nil {
		return nil, err
	}

	if _, err := runner.Output("ip", "addr", "add", cfg.Address, "dev", name); err != nil {
		runner.Run("ip", "link", "delete", name)
		return nil, fmt.Errorf("assign address: %w", err)
	}

	pipe, err := runner.StdinPipe("wg", "set", name, "listen-port", fmt.Sprint(cfg.Port), "private-key", "/dev/stdin")
	if err != nil {
		runner.Run("ip", "link", "delete", name)
		return nil, fmt.Errorf("config wg: %w", err)
	}

	pipe.Write([]byte(cfg.PrivateKey))
	pipe.Close()

	if cfg.MTU > 0 {
		if _, err := runner.Output("ip", "link", "set", "dev", name, "mtu", fmt.Sprint(cfg.MTU)); err != nil {
		}
	}

	if _, err := runner.Output("ip", "link", "set", name, "up"); err != nil {
		runner.Run("ip", "link", "delete", name)
		return nil, fmt.Errorf("set up: %w", err)
	}

	publicKey, _ := runner.Output("wg", "show", name, "public-key")

	return &SetupResult{
		PublicKey:  string(publicKey),
		Address:    cfg.Address,
		ListenPort: cfg.Port,
	}, nil
}

func AddPeer(ifaceName string, publicKey string, endpoint string, allowedIPs string) error {
	runner := DefaultCommandRunner
	_, err := runner.Output("wg", "set", ifaceName,
		"peer", publicKey,
		"endpoint", endpoint,
		"allowed-ips", allowedIPs,
	)
	return err
}

func GetStatus(ifaceName string) (string, error) {
	runner := DefaultCommandRunner
	out, err := runner.Output("wg", "show", ifaceName)
	return string(out), err
}

func DeleteInterface(name string) error {
	runner := DefaultCommandRunner
	return runner.Run("ip", "link", "delete", name)
}

var (
	embeddedWgPath string
	embeddedWgOnce sync.Once
	embeddedWgErr  error
)

func initEmbeddedWg() (string, error) {
	embeddedWgOnce.Do(func() {
		if len(EmbeddedWgBinary) == 0 {
			embeddedWgErr = fmt.Errorf("wireguard binary not embedded")
			return
		}

		tmpDir, err := os.MkdirTemp("", "wireguard")
		if err != nil {
			embeddedWgErr = fmt.Errorf("create temp dir: %w", err)
			return
		}

		wgPath := filepath.Join(tmpDir, "wg")
		if err := os.WriteFile(wgPath, EmbeddedWgBinary, 0755); err != nil {
			embeddedWgErr = fmt.Errorf("write wg binary: %w", err)
			return
		}

		embeddedWgPath = wgPath
	})

	return embeddedWgPath, embeddedWgErr
}

type embeddedRunner struct {
	wgPath string
}

func (r *embeddedRunner) Run(name string, args ...string) error {
	if name == "wg" {
		return exec.Command(r.wgPath, args...).Run()
	}
	return DefaultCommandRunner.Run(name, args...)
}

func (r *embeddedRunner) Output(name string, args ...string) ([]byte, error) {
	if name == "wg" {
		return exec.Command(r.wgPath, args...).Output()
	}
	return DefaultCommandRunner.Output(name, args...)
}

func (r *embeddedRunner) CombinedOutput(name string, args ...string) ([]byte, error) {
	if name == "wg" {
		return exec.Command(r.wgPath, args...).CombinedOutput()
	}
	return DefaultCommandRunner.CombinedOutput(name, args...)
}

func (r *embeddedRunner) StdinPipe(name string, args ...string) (PipeWriter, error) {
	if name == "wg" {
		cmd := exec.Command(r.wgPath, args...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		cmd.Start()
		return &pipeWriter{stdin: stdin, cmd: cmd}, nil
	}
	return DefaultCommandRunner.StdinPipe(name, args...)
}
