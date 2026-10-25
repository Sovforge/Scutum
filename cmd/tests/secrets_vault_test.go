package tests

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"scutum/cmd/internal/handlers"
)

func TestValidateSecretName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"db-password", true},
		{"prod/db/password", true},
		{"prod_db.password-1", true},
		{"", false},
		{"/leading-slash", false},
		{"trailing-slash/", false},
		{"double//slash", false},
		{"../etc/passwd", false},
		{"has space", false},
		{"has\ttab", false},
		{"quote\"here", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := handlers.ValidateSecretName(tt.name); got != tt.valid {
				t.Fatalf("ValidateSecretName(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

type fakeSecretsStore struct {
	values map[string][]byte
}

func (f *fakeSecretsStore) GetAppSecretValue(_ context.Context, name string) ([]byte, error) {
	v, ok := f.values[name]
	if !ok {
		return nil, errors.New("secret not found")
	}
	return v, nil
}

func TestResolveSecretEnvRefs(t *testing.T) {
	store := &fakeSecretsStore{values: map[string][]byte{"db/password": []byte("hunter2")}}

	t.Run("resolves a matching reference", func(t *testing.T) {
		env := []string{"PLAIN=value", "DB_PASSWORD=secret://db/password"}
		out, used, err := handlers.ResolveSecretEnvRefs(context.Background(), store, env)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"PLAIN=value", "DB_PASSWORD=hunter2"}
		for i := range want {
			if out[i] != want[i] {
				t.Fatalf("out[%d] = %q, want %q", i, out[i], want[i])
			}
		}
		if len(used) != 1 || used[0] != "db/password" {
			t.Fatalf("used = %v", used)
		}
	})

	t.Run("leaves plain values untouched", func(t *testing.T) {
		env := []string{"PLAIN=value"}
		out, used, err := handlers.ResolveSecretEnvRefs(context.Background(), store, env)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out[0] != "PLAIN=value" || len(used) != 0 {
			t.Fatalf("got out=%v used=%v", out, used)
		}
	})

	t.Run("errors on unknown secret", func(t *testing.T) {
		env := []string{"X=secret://does-not-exist"}
		_, _, err := handlers.ResolveSecretEnvRefs(context.Background(), store, env)
		if err == nil {
			t.Fatal("expected error for unknown secret")
		}
	})
}

func TestResolveSecretRefsText(t *testing.T) {
	store := &fakeSecretsStore{values: map[string][]byte{"api/key": []byte("s3cr3t-val")}}

	t.Run("resolves references embedded in YAML", func(t *testing.T) {
		yaml := []byte("env:\n  - name: API_KEY\n    value: secret://api/key\n")
		out, used, err := handlers.ResolveSecretRefsText(context.Background(), store, yaml)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Contains(out, []byte("value: s3cr3t-val")) {
			t.Fatalf("got %q, missing resolved value", out)
		}
		if len(used) != 1 || used[0] != "api/key" {
			t.Fatalf("used = %v", used)
		}
	})

	t.Run("passes through text with no references", func(t *testing.T) {
		yaml := []byte("env:\n  - name: PLAIN\n    value: hello\n")
		out, used, err := handlers.ResolveSecretRefsText(context.Background(), store, yaml)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(out) != string(yaml) || len(used) != 0 {
			t.Fatalf("got out=%q used=%v", out, used)
		}
	})

	t.Run("errors on unknown secret", func(t *testing.T) {
		yaml := []byte("value: secret://does-not-exist\n")
		_, _, err := handlers.ResolveSecretRefsText(context.Background(), store, yaml)
		if err == nil {
			t.Fatal("expected error for unknown secret")
		}
	})
}
