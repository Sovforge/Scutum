package utils

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// IsWireGuardAvailable reports whether WireGuard can be used on this host —
// either via the kernel module or a wireguard-go binary in PATH.
func IsWireGuardAvailable() bool {
	if _, err := os.Stat("/sys/module/wireguard"); err == nil {
		return true
	}
	_, err := exec.LookPath("wireguard-go")
	return err == nil
}

// TryInstallWireGuardGo attempts to install the wireguard-go package using
// whichever system package manager is available. It tries the most common
// Linux package managers in order and returns nil on the first success.
func TryInstallWireGuardGo(ctx context.Context) error {
	type entry struct {
		bin  string
		args []string
	}
	candidates := []entry{
		{"apt-get", []string{"install", "-y", "wireguard-go"}},
		{"apk", []string{"add", "wireguard-go"}},
		{"dnf", []string{"install", "-y", "wireguard-go"}},
		{"yum", []string{"install", "-y", "wireguard-go"}},
		{"pacman", []string{"-S", "--noconfirm", "wireguard-go"}},
		{"zypper", []string{"install", "-y", "wireguard-go"}},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.bin); err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, c.bin, c.args...)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("wireguard-go auto-install failed: no compatible package manager succeeded")
}

// ErrWireGuardUnavailable is returned when neither the kernel module nor
// wireguard-go is available on the host. Callers can check for it with
// errors.Is to decide whether to degrade gracefully.
var ErrWireGuardUnavailable = errors.New("wireguard unavailable")

// DerivePublicKey computes the Curve25519 public key for a WireGuard private
// key (base64-encoded, 32 bytes). Uses the standard library — no wg binary
// or kernel module required.
func DerivePublicKey(privateKeyB64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privateKeyB64))
	if err != nil || len(raw) != 32 {
		return "", fmt.Errorf("invalid WireGuard private key")
	}
	priv, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return "", fmt.Errorf("derive public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()), nil
}

// ensureWireGuardInterface creates the named WireGuard interface. It tries the
// kernel module first (ip link add ... type wireguard) and falls back to the
// wireguard-go userspace daemon when the kernel module is not available.
func ensureWireGuardInterface(runner CommandRunner, name string) error {
	if _, err := runner.Output("ip", "link", "add", name, "type", "wireguard"); err == nil {
		return nil
	}

	// Kernel module unavailable; try wireguard-go userspace implementation.
	wgGoPath, err := exec.LookPath("wireguard-go")
	if err != nil {
		return fmt.Errorf("%w: kernel module not loaded and wireguard-go not found in PATH (install wireguard-go or load the module with: modprobe wireguard)", ErrWireGuardUnavailable)
	}

	cmd := exec.Command(wgGoPath, name)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: start wireguard-go: %v", ErrWireGuardUnavailable, err)
	}

	// wireguard-go may daemonize (exits the parent quickly) or run in the
	// foreground. Wait briefly: an immediate non-zero exit means failure.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	select {
	case err := <-exited:
		if err != nil {
			return fmt.Errorf("%w: wireguard-go failed: %v", ErrWireGuardUnavailable, err)
		}
		// Exited cleanly — daemonized successfully.
	case <-time.After(500 * time.Millisecond):
		// Still running in foreground mode; detach and continue.
		_ = cmd.Process.Release()
	}

	// Poll for the interface to appear (up to 2 seconds).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := runner.Output("ip", "link", "show", name); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("%w: wireguard-go started but interface %q did not appear within 2s", ErrWireGuardUnavailable, name)
}
