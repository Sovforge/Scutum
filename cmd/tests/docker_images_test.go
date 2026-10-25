package tests

import (
	"testing"

	"scutum/cmd/internal/handlers"
)

func TestSplitImageRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantRepo string
		wantTag  string
	}{
		{"nginx", "nginx", "latest"},
		{"nginx:1.25", "nginx", "1.25"},
		{"nginx:latest", "nginx", "latest"},
		{"org/app", "org/app", "latest"},
		{"org/app:1.2.3", "org/app", "1.2.3"},
		{"ghcr.io/org/app:1.2.3", "ghcr.io/org/app", "1.2.3"},
		{"myregistry:5000/nginx", "myregistry:5000/nginx", "latest"},
		{"myregistry:5000/nginx:1.25", "myregistry:5000/nginx", "1.25"},
		{"myregistry:5000/org/app:1.2.3", "myregistry:5000/org/app", "1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			repo, tag := handlers.SplitImageRef(tt.ref)
			if repo != tt.wantRepo || tag != tt.wantTag {
				t.Fatalf("SplitImageRef(%q) = (%q, %q), want (%q, %q)", tt.ref, repo, tag, tt.wantRepo, tt.wantTag)
			}
		})
	}
}
