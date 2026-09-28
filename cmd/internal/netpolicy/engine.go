// Package netpolicy translates stored network policy rules into iptables
// commands and applies them to a single, dedicated chain hooked only off
// the WireGuard interface's own forwarded traffic — never INPUT, OUTPUT, or
// any other interface. A misconfigured policy (including default-deny) can
// therefore never lock out the hub's own management access, SSH, or any
// other host service: only mesh-to-mesh (wg0-to-wg0) traffic passes through
// SCUTUM_POLICY at all.
package netpolicy

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"scutum/cmd/internal/store"
)

// ChainName is the dedicated iptables chain every policy rule is written
// into. Never referenced from INPUT/OUTPUT — only hooked off FORWARD
// traffic entering and leaving the WireGuard interface.
const ChainName = "SCUTUM_POLICY"

// LogPrefix marks packets dropped by a policy so the violation tailer
// (violations.go) can find them in the kernel log and nothing else can be
// mistaken for one.
const LogPrefix = "SCUTUM_DENY: "

// Rule is a policy already resolved to concrete mesh IPs — no more
// node/group lookups needed to turn it into iptables arguments.
type Rule struct {
	Priority int      `json:"priority"`
	Action   string   `json:"action"`   // "allow" | "deny"
	Protocol string   `json:"protocol"` // "tcp" | "udp" | "icmp" | "any" | ""
	Port     string   `json:"port"`     // "" (any), "22", or "1000-2000"
	SrcIPs   []string `json:"src_ips"`
	DstIPs   []string `json:"dst_ips"`
}

// Runner executes a single command and returns its combined output — the
// same shape as utils.CommandRunner.CombinedOutput, kept separate so this
// package doesn't need to import cmd/internal/utils just for the interface.
type Runner interface {
	CombinedOutput(name string, args ...string) ([]byte, error)
}

// IsAvailable reports whether the iptables binary is present on this host.
// Mirrors utils.IsWireGuardAvailable's shape.
func IsAvailable() bool {
	_, err := exec.LookPath("iptables")
	return err == nil
}

// Resolver turns a policy's src/dst target ("any"/"node"/"group" + ID) into
// zero or more concrete mesh IPs. Zero IPs with targetType "any" means
// unrestricted (no -s/-d filter); zero IPs with "node"/"group" and a
// non-nil error means the target couldn't be resolved (unknown ID, or a
// node with no established WireGuard peer yet).
type Resolver interface {
	ResolveTarget(ctx context.Context, targetType, targetID string) ([]string, error)
}

// BuildRules resolves every enabled policy (already ordered by priority —
// see store.ListNetworkPolicies) into concrete Rules. A policy whose src or
// dst can't be resolved is skipped and reported in the returned map rather
// than aborting the whole set, so one bad reference doesn't take down
// enforcement for every other policy.
func BuildRules(ctx context.Context, resolver Resolver, policies []store.NetworkPolicy) ([]Rule, map[string]error) {
	var rules []Rule
	errs := map[string]error{}
	for _, p := range policies {
		if !p.Enabled {
			continue
		}
		srcIPs, err := resolver.ResolveTarget(ctx, p.SrcType, p.SrcID)
		if err != nil {
			errs[p.ID] = fmt.Errorf("resolve source: %w", err)
			continue
		}
		dstIPs, err := resolver.ResolveTarget(ctx, p.DstType, p.DstID)
		if err != nil {
			errs[p.ID] = fmt.Errorf("resolve destination: %w", err)
			continue
		}
		rules = append(rules, Rule{
			Priority: p.Priority, Action: p.Action, Protocol: p.Protocol, Port: p.Port,
			SrcIPs: srcIPs, DstIPs: dstIPs,
		})
	}
	return rules, errs
}

// Apply flushes SCUTUM_POLICY and rewrites it from scratch to match rules
// (already in priority order) plus defaultDeny. Flush-then-rebuild, rather
// than incremental insert/delete, is deliberate: it's simple to reason
// about and idempotent, and its only failure mode is a brief fully-open
// window (fail open), never a stuck-closed one.
func Apply(runner Runner, iface string, rules []Rule, defaultDeny bool) error {
	if err := ensureChain(runner); err != nil {
		return err
	}
	if err := ensureHook(runner, iface); err != nil {
		return err
	}
	if out, err := runner.CombinedOutput("iptables", "-F", ChainName); err != nil {
		return fmt.Errorf("flush %s: %w: %s", ChainName, err, out)
	}

	for _, rule := range rules {
		for _, args := range rule.iptablesArgs() {
			if out, err := runner.CombinedOutput("iptables", args...); err != nil {
				return fmt.Errorf("apply rule (priority %d): %w: %s", rule.Priority, err, out)
			}
		}
	}

	if defaultDeny {
		logArgs := []string{"-A", ChainName, "-j", "LOG", "--log-prefix", LogPrefix + "DEFAULT: ", "--log-level", "4"}
		if out, err := runner.CombinedOutput("iptables", logArgs...); err != nil {
			return fmt.Errorf("apply default-deny log rule: %w: %s", err, out)
		}
		if out, err := runner.CombinedOutput("iptables", "-A", ChainName, "-j", "DROP"); err != nil {
			return fmt.Errorf("apply default-deny drop rule: %w: %s", err, out)
		}
	}
	return nil
}

func ensureChain(runner Runner) error {
	out, err := runner.CombinedOutput("iptables", "-N", ChainName)
	if err != nil && !strings.Contains(string(out), "already exists") {
		return fmt.Errorf("create chain %s: %w: %s", ChainName, err, out)
	}
	return nil
}

// ensureHook adds the FORWARD -> SCUTUM_POLICY jump exactly once, scoped to
// traffic entering AND leaving the WireGuard interface — mesh-to-mesh
// traffic only. -C (check) first so re-running Apply never duplicates it.
func ensureHook(runner Runner, iface string) error {
	if _, err := runner.CombinedOutput("iptables", "-C", "FORWARD", "-i", iface, "-o", iface, "-j", ChainName); err == nil {
		return nil
	}
	if out, err := runner.CombinedOutput("iptables", "-A", "FORWARD", "-i", iface, "-o", iface, "-j", ChainName); err != nil {
		return fmt.Errorf("hook %s into FORWARD: %w: %s", ChainName, err, out)
	}
	return nil
}

// iptablesArgs renders one Rule into the ordered sequence of `iptables`
// argument lists needed to apply it — an "allow" is one ACCEPT rule per
// src/dst pair; a "deny" is a LOG rule immediately followed by a DROP rule
// sharing the same match, so the violation tailer can see what was dropped.
func (rule Rule) iptablesArgs() [][]string {
	srcs := rule.SrcIPs
	if len(srcs) == 0 {
		srcs = []string{""}
	}
	dsts := rule.DstIPs
	if len(dsts) == 0 {
		dsts = []string{""}
	}

	var out [][]string
	for _, src := range srcs {
		for _, dst := range dsts {
			match := matchArgs(src, dst, rule.Protocol, rule.Port)
			if rule.Action == "deny" {
				out = append(out, withArgs(match, "-j", "LOG", "--log-prefix", LogPrefix, "--log-level", "4"))
				out = append(out, withArgs(match, "-j", "DROP"))
			} else {
				out = append(out, withArgs(match, "-j", "ACCEPT"))
			}
		}
	}
	return out
}

func matchArgs(src, dst, protocol, port string) []string {
	var args []string
	if src != "" {
		args = append(args, "-s", src)
	}
	if dst != "" {
		args = append(args, "-d", dst)
	}
	if protocol != "" && protocol != "any" {
		args = append(args, "-p", protocol)
		if port != "" && protocol != "icmp" {
			args = append(args, "--dport", port)
		}
	}
	return args
}

func withArgs(match []string, extra ...string) []string {
	args := make([]string, 0, len(match)+len(extra)+2)
	args = append(args, "-A", ChainName)
	args = append(args, match...)
	args = append(args, extra...)
	return args
}
