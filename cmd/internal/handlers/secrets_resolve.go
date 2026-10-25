package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// secretsValueStore is the minimal surface deploy-time secret resolution
// needs — satisfied by *store.Store, and small enough to fake in tests.
type secretsValueStore interface {
	GetAppSecretValue(ctx context.Context, name string) ([]byte, error)
}

// secretRefRe matches a "secret://<name>" reference. The name charset here
// must stay in sync with ValidateSecretName so anything a user could
// actually create is also resolvable.
var secretRefRe = regexp.MustCompile(`secret://([A-Za-z0-9_.\-/]+)`)

// ResolveSecretEnvRefs replaces "secret://name" values in a Docker-style
// "KEY=VALUE" env slice with the real secret value, decrypted on demand.
// It never mutates the input slice. Returns the names of any secrets it
// resolved, for audit logging by the caller.
func ResolveSecretEnvRefs(ctx context.Context, secrets secretsValueStore, env []string) ([]string, []string, error) {
	if secrets == nil || len(env) == 0 {
		return env, nil, nil
	}
	out := make([]string, len(env))
	var used []string
	for i, kv := range env {
		key, val, found := strings.Cut(kv, "=")
		if !found || !strings.HasPrefix(val, "secret://") {
			out[i] = kv
			continue
		}
		name := strings.TrimPrefix(val, "secret://")
		plain, err := secrets.GetAppSecretValue(ctx, name)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve secret %q: %w", name, err)
		}
		out[i] = key + "=" + string(plain)
		used = append(used, name)
	}
	return out, used, nil
}

// ResolveSecretRefsText replaces every "secret://name" occurrence found
// anywhere in raw text (a Compose or Kubernetes YAML manifest) with the
// real secret value. Used because these deploy paths never structurally
// parse the manifest — they shell it straight to `docker compose`/`kubectl`.
func ResolveSecretRefsText(ctx context.Context, secrets secretsValueStore, body []byte) ([]byte, []string, error) {
	if secrets == nil || !secretRefRe.Match(body) {
		return body, nil, nil
	}

	var used []string
	var resolveErr error
	out := secretRefRe.ReplaceAllFunc(body, func(match []byte) []byte {
		if resolveErr != nil {
			return match
		}
		name := string(secretRefRe.FindSubmatch(match)[1])
		plain, err := secrets.GetAppSecretValue(ctx, name)
		if err != nil {
			resolveErr = fmt.Errorf("resolve secret %q: %w", name, err)
			return match
		}
		used = append(used, name)
		return plain
	})
	if resolveErr != nil {
		return nil, nil, resolveErr
	}
	return out, used, nil
}
