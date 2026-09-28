package netpolicy

import (
	"bufio"
	"context"
	"os/exec"
	"regexp"
)

// Violation is one packet dropped by a "deny" policy or by default-deny,
// parsed from the kernel's netfilter LOG line.
type Violation struct {
	Src      string
	Dst      string
	Protocol string
	Port     string
	Raw      string
}

var (
	srcRe   = regexp.MustCompile(`SRC=(\S+)`)
	dstRe   = regexp.MustCompile(`DST=(\S+)`)
	protoRe = regexp.MustCompile(`PROTO=(\S+)`)
	dptRe   = regexp.MustCompile(`DPT=(\S+)`)
)

func parseViolation(line string) Violation {
	v := Violation{Raw: line}
	if m := srcRe.FindStringSubmatch(line); m != nil {
		v.Src = m[1]
	}
	if m := dstRe.FindStringSubmatch(line); m != nil {
		v.Dst = m[1]
	}
	if m := protoRe.FindStringSubmatch(line); m != nil {
		v.Protocol = m[1]
	}
	if m := dptRe.FindStringSubmatch(line); m != nil {
		v.Port = m[1]
	}
	return v
}

// IsViolationTailingAvailable reports whether journalctl is present — the
// only mechanism used to read kernel LOG lines. Violation logging degrades
// to a no-op without it; nothing else about policy enforcement depends on
// this being available.
func IsViolationTailingAvailable() bool {
	_, err := exec.LookPath("journalctl")
	return err == nil
}

// TailViolations streams SCUTUM_DENY kernel log lines (from the LOG rules
// Apply writes ahead of every DROP) and calls onViolation for each one,
// until ctx is canceled. Best-effort: a journalctl failure is returned to
// the caller to log, never panics, and never blocks startup — callers
// should run this in its own goroutine and just log a warning on error.
func TailViolations(ctx context.Context, onViolation func(Violation)) error {
	cmd := exec.CommandContext(ctx, "journalctl", "-kf", "-g", LogPrefix, "--no-pager", "-o", "cat")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		onViolation(parseViolation(scanner.Text()))
	}

	// cmd.Wait() returns a "signal: killed" error when ctx is canceled —
	// that's the normal shutdown path, not a real failure.
	if waitErr := cmd.Wait(); waitErr != nil && ctx.Err() == nil {
		return waitErr
	}
	return nil
}
