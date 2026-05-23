//go:build !embed_wireguard
// +build !embed_wireguard

package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os/exec"
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

func UpdatePeerEndpoint(ifaceName, publicKey, endpoint string) error {
	runner := DefaultCommandRunner
	out, err := runner.CombinedOutput("wg", "set", ifaceName, "peer", publicKey, "endpoint", endpoint)
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

func AddPeer(ifaceName string, publicKey string, endpoint string, allowedIPs string, keepalive int) error {
	runner := DefaultCommandRunner
	args := []string{"set", ifaceName, "peer", publicKey, "endpoint", endpoint, "allowed-ips", allowedIPs}
	if keepalive > 0 {
		args = append(args, "persistent-keepalive", fmt.Sprint(keepalive))
	}
	out, err := runner.CombinedOutput("wg", args...)
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

func GetStatus(ifaceName string) (string, error) {
	runner := DefaultCommandRunner
	out, err := runner.Output("wg", "show", ifaceName)
	return string(out), err
}

func GetDump(ifaceName string) (string, error) {
	runner := DefaultCommandRunner
	out, err := runner.Output("wg", "show", ifaceName, "dump")
	return string(out), err
}

func DeleteInterface(name string) error {
	runner := DefaultCommandRunner
	return runner.Run("ip", "link", "delete", name)
}
