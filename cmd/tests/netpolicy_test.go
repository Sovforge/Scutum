package tests

import (
	"context"
	"errors"
	"strings"
	"testing"

	"scutum/cmd/internal/netpolicy"
	"scutum/cmd/internal/store"
)

// fakeRunner records every command issued and lets tests script specific
// failures (e.g. "-C ... already exists" style checks).
type fakeRunner struct {
	calls  [][]string
	failOn func(args []string) ([]byte, error)
}

func (f *fakeRunner) CombinedOutput(name string, args ...string) ([]byte, error) {
	full := append([]string{name}, args...)
	f.calls = append(f.calls, full)
	if f.failOn != nil {
		return f.failOn(args)
	}
	return nil, nil
}

func TestNetpolicyApplyHooksChainOnce(t *testing.T) {
	runner := &fakeRunner{
		failOn: func(args []string) ([]byte, error) {
			// -C (check) always fails the first time so -A gets exercised.
			if len(args) > 0 && args[0] == "-C" {
				return nil, errors.New("no such rule")
			}
			return nil, nil
		},
	}
	if err := netpolicy.Apply(runner, "wg0", nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var joined []string
	for _, c := range runner.calls {
		joined = append(joined, strings.Join(c, " "))
	}
	full := strings.Join(joined, "\n")

	if !strings.Contains(full, "-N SCUTUM_POLICY") {
		t.Errorf("expected chain creation, got:\n%s", full)
	}
	if !strings.Contains(full, "-C FORWARD -i wg0 -o wg0 -j SCUTUM_POLICY") {
		t.Errorf("expected hook check, got:\n%s", full)
	}
	if !strings.Contains(full, "-A FORWARD -i wg0 -o wg0 -j SCUTUM_POLICY") {
		t.Errorf("expected hook insert after failed check, got:\n%s", full)
	}
	if !strings.Contains(full, "-F SCUTUM_POLICY") {
		t.Errorf("expected flush, got:\n%s", full)
	}
	// With no rules and defaultDeny=false, nothing else should touch FORWARD
	// or any interface other than wg0 — confirms the "never lock out the
	// hub" scoping.
	for _, c := range runner.calls {
		for _, a := range c {
			if a == "INPUT" || a == "OUTPUT" {
				t.Fatalf("Apply must never touch INPUT/OUTPUT, got call: %v", c)
			}
		}
	}
}

func TestNetpolicyApplySkipsHookWhenAlreadyPresent(t *testing.T) {
	runner := &fakeRunner{} // -C always succeeds (nil error)
	if err := netpolicy.Apply(runner, "wg0", nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range runner.calls {
		if len(c) >= 2 && c[1] == "-A" && strings.Join(c, " ") != "" && strings.Contains(strings.Join(c, " "), "FORWARD") {
			t.Fatalf("hook should not be re-inserted when -C already succeeds, got: %v", c)
		}
	}
}

func TestNetpolicyApplyToleratesExistingChain(t *testing.T) {
	runner := &fakeRunner{
		failOn: func(args []string) ([]byte, error) {
			if len(args) > 0 && args[0] == "-N" {
				return []byte("iptables: Chain already exists."), errors.New("exit status 1")
			}
			return nil, nil
		},
	}
	if err := netpolicy.Apply(runner, "wg0", nil, false); err != nil {
		t.Fatalf("expected 'already exists' to be tolerated, got: %v", err)
	}
}

func TestNetpolicyApplyRendersAllowAndDenyRules(t *testing.T) {
	runner := &fakeRunner{}
	rules := []netpolicy.Rule{
		{Priority: 10, Action: "allow", Protocol: "tcp", Port: "22", SrcIPs: []string{"10.0.0.2"}, DstIPs: []string{"10.0.0.3"}},
		{Priority: 20, Action: "deny", Protocol: "any", SrcIPs: nil, DstIPs: []string{"10.0.0.9"}},
	}
	if err := netpolicy.Apply(runner, "wg0", rules, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var joined []string
	for _, c := range runner.calls {
		joined = append(joined, strings.Join(c, " "))
	}
	full := strings.Join(joined, "\n")

	if !strings.Contains(full, "-s 10.0.0.2 -d 10.0.0.3 -p tcp --dport 22 -j ACCEPT") {
		t.Errorf("missing rendered allow rule, got:\n%s", full)
	}
	if !strings.Contains(full, "-d 10.0.0.9 -j LOG --log-prefix SCUTUM_DENY:  --log-level 4") {
		t.Errorf("missing deny LOG rule, got:\n%s", full)
	}
	if !strings.Contains(full, "-d 10.0.0.9 -j DROP") {
		t.Errorf("missing deny DROP rule, got:\n%s", full)
	}
	// default-deny appends its own final LOG+DROP with no match.
	if !strings.Contains(full, "-j LOG --log-prefix SCUTUM_DENY: DEFAULT:  --log-level 4") {
		t.Errorf("missing default-deny LOG rule, got:\n%s", full)
	}
}

func TestNetpolicyIptablesArgsExpandsGroupCrossProduct(t *testing.T) {
	rule := netpolicy.Rule{
		Action: "allow", Protocol: "any",
		SrcIPs: []string{"10.0.0.1", "10.0.0.2"},
		DstIPs: []string{"10.0.0.9"},
	}
	runner := &fakeRunner{}
	if err := netpolicy.Apply(runner, "wg0", []netpolicy.Rule{rule}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var acceptCount int
	for _, c := range runner.calls {
		joined := strings.Join(c, " ")
		if strings.Contains(joined, "-j ACCEPT") {
			acceptCount++
		}
	}
	if acceptCount != 2 {
		t.Fatalf("expected 2 ACCEPT rules (one per src IP), got %d", acceptCount)
	}
}

// fakeResolver implements netpolicy.Resolver for BuildRules tests.
type fakeResolver struct {
	ips map[string][]string
	err map[string]error
}

func (f *fakeResolver) ResolveTarget(_ context.Context, targetType, targetID string) ([]string, error) {
	if targetType == "" || targetType == "any" {
		return nil, nil
	}
	key := targetType + ":" + targetID
	if err, ok := f.err[key]; ok {
		return nil, err
	}
	return f.ips[key], nil
}

func TestBuildRulesSkipsUnresolvablePolicyButKeepsOthers(t *testing.T) {
	resolver := &fakeResolver{
		ips: map[string][]string{"node:n1": {"10.0.0.5"}},
		err: map[string]error{"group:missing": errors.New("group not found")},
	}
	policies := []store.NetworkPolicy{
		{ID: "p1", Enabled: true, Priority: 1, Action: "allow", Protocol: "any", SrcType: "node", SrcID: "n1", DstType: "any"},
		{ID: "p2", Enabled: true, Priority: 2, Action: "deny", Protocol: "any", SrcType: "group", SrcID: "missing", DstType: "any"},
		{ID: "p3", Enabled: false, Priority: 3, Action: "allow", Protocol: "any", SrcType: "any", DstType: "any"},
	}

	rules, errs := netpolicy.BuildRules(context.Background(), resolver, policies)

	if len(rules) != 1 {
		t.Fatalf("expected 1 resolved rule (p3 disabled, p2 unresolvable), got %d: %+v", len(rules), rules)
	}
	if rules[0].SrcIPs[0] != "10.0.0.5" {
		t.Fatalf("unexpected resolved rule: %+v", rules[0])
	}
	if _, ok := errs["p2"]; !ok {
		t.Fatalf("expected p2's resolve error to be reported, got errs=%v", errs)
	}
	if _, ok := errs["p1"]; ok {
		t.Fatalf("p1 should have resolved cleanly, but got an error")
	}
}
