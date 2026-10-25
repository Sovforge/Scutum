package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/store"
)

func TestSafeJoin(t *testing.T) {
	root := "/data/distributed-files"

	tests := []struct {
		name    string
		dest    string
		wantErr bool
	}{
		{"plain relative path", "app/config.txt", false},
		{"nested relative path", "a/b/c/d.conf", false},
		{"empty", "", true},
		{"absolute path", "/etc/passwd", true},
		{"leading traversal", "../etc/passwd", true},
		{"embedded traversal", "app/../../etc/passwd", true},
		{"traversal that stays under root nets out fine", "app/../config.txt", false},
		{"just dotdot", "..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handlers.SafeJoin(root, tt.dest)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SafeJoin(%q) = %q, nil; want error", tt.dest, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("SafeJoin(%q) unexpected error: %v", tt.dest, err)
			}
			if !strings.HasPrefix(got, filepath.Clean(root)+string(os.PathSeparator)) && got != filepath.Clean(root) {
				t.Fatalf("SafeJoin(%q) = %q escapes root %q", tt.dest, got, root)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	node := store.NodeRecord{ID: "n1", Name: "edge-01", Address: "10.100.0.5:8081"}

	t.Run("built-in node vars", func(t *testing.T) {
		out, err := handlers.RenderTemplate([]byte("ip={{.NodeIP}} name={{.NodeName}} id={{.NodeID}}"), node, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "ip=10.100.0.5 name=edge-01 id=n1"
		if string(out) != want {
			t.Fatalf("got %q, want %q", out, want)
		}
	})

	t.Run("caller-supplied vars", func(t *testing.T) {
		out, err := handlers.RenderTemplate([]byte("env={{.Environment}}"), node, map[string]string{"Environment": "production"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(out) != "env=production" {
			t.Fatalf("got %q", out)
		}
	})

	t.Run("plain text with no directives passes through unchanged", func(t *testing.T) {
		out, err := handlers.RenderTemplate([]byte("just plain text, no templating here"), node, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(out) != "just plain text, no templating here" {
			t.Fatalf("got %q", out)
		}
	})

	t.Run("unknown key errors instead of silently emitting <no value>", func(t *testing.T) {
		_, err := handlers.RenderTemplate([]byte("{{.NotARealField}}"), node, nil)
		if err == nil {
			t.Fatal("expected an error for an undefined template field, got nil")
		}
	})
}
