package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"scutum/cmd/internal/gitops"
	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"
)

// initTestRepo creates a real local git repo with the given file content
// committed, so Fetch can be exercised against a real `git clone`/fetch
// without any network access.
func initTestRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "-A")
	run("commit", "-m", "initial")
	return dir
}

func TestGitopsFetchClonesAndDetectsChange(t *testing.T) {
	origin := initTestRepo(t, map[string]string{"docker-compose.yml": "version: '3'\nservices:\n  web:\n    image: nginx\n"})
	workDir := filepath.Join(t.TempDir(), "clone")

	repo := &utils.GitRepo{URL: origin, Branch: "main", LocalDir: workDir}

	result, err := gitops.Fetch(repo, workDir, "docker-compose.yml", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Changed {
		t.Fatal("expected Changed=true on first fetch (previousContentHash empty)")
	}
	if result.CommitSHA == "" {
		t.Fatal("expected a non-empty commit SHA")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty manifest content")
	}

	// Fetching again with the same previous hash should report unchanged.
	result2, err := gitops.Fetch(repo, workDir, "docker-compose.yml", result.ContentHash)
	if err != nil {
		t.Fatalf("unexpected error on second fetch: %v", err)
	}
	if result2.Changed {
		t.Fatal("expected Changed=false when content hasn't moved")
	}
	if result2.CommitSHA != result.CommitSHA {
		t.Fatalf("commit SHA changed unexpectedly: %q vs %q", result.CommitSHA, result2.CommitSHA)
	}
}

func TestGitopsFetchRejectsPathTraversal(t *testing.T) {
	origin := initTestRepo(t, map[string]string{"docker-compose.yml": "services: {}\n"})
	workDir := filepath.Join(t.TempDir(), "clone")
	repo := &utils.GitRepo{URL: origin, Branch: "main", LocalDir: workDir}

	// A manifest path trying to escape the repo root should resolve back
	// inside workDir (via filepath.Clean against "/") and fail to read
	// rather than reading something outside the repo.
	_, err := gitops.Fetch(repo, workDir, "../../../etc/passwd", "")
	if err == nil {
		t.Fatal("expected an error reading a path-traversal manifest path")
	}
}

func TestGitopsFetchDetectsRealContentChange(t *testing.T) {
	origin := initTestRepo(t, map[string]string{"app.yaml": "replicas: 1\n"})
	workDir := filepath.Join(t.TempDir(), "clone")
	repo := &utils.GitRepo{URL: origin, Branch: "main", LocalDir: workDir}

	first, err := gitops.Fetch(repo, workDir, "app.yaml", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Push a real change to the origin.
	if err := os.WriteFile(filepath.Join(origin, "app.yaml"), []byte("replicas: 3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = origin
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("commit", "-am", "bump replicas")

	second, err := gitops.Fetch(repo, workDir, "app.yaml", first.ContentHash)
	if err != nil {
		t.Fatalf("unexpected error on second fetch: %v", err)
	}
	if !second.Changed {
		t.Fatal("expected Changed=true after a real upstream content change")
	}
	if second.CommitSHA == first.CommitSHA {
		t.Fatal("expected a new commit SHA after the upstream change")
	}
	if string(second.Content) != "replicas: 3\n" {
		t.Fatalf("got content %q, want the updated manifest", second.Content)
	}
}

func TestDueForSync(t *testing.T) {
	now := time.Now().UTC()

	t.Run("never synced is always due", func(t *testing.T) {
		src := store.GitOpsSource{PollIntervalSeconds: 60}
		if !handlers.DueForSync(src) {
			t.Fatal("expected a never-synced source to be due")
		}
	})

	t.Run("synced recently is not due", func(t *testing.T) {
		src := store.GitOpsSource{PollIntervalSeconds: 300, LastSyncedAt: now.Add(-10 * time.Second).Format(time.RFC3339)}
		if handlers.DueForSync(src) {
			t.Fatal("expected a recently-synced source (10s ago, 300s interval) to not be due yet")
		}
	})

	t.Run("synced past interval is due", func(t *testing.T) {
		src := store.GitOpsSource{PollIntervalSeconds: 60, LastSyncedAt: now.Add(-120 * time.Second).Format(time.RFC3339)}
		if !handlers.DueForSync(src) {
			t.Fatal("expected a source synced 120s ago with a 60s interval to be due")
		}
	})

	t.Run("unparseable timestamp fails open (due)", func(t *testing.T) {
		src := store.GitOpsSource{PollIntervalSeconds: 60, LastSyncedAt: "not-a-timestamp"}
		if !handlers.DueForSync(src) {
			t.Fatal("expected an unparseable last_synced_at to be treated as due")
		}
	})
}

func TestVerifyGitHubSignature(t *testing.T) {
	secret := []byte("shh-its-a-secret")
	body := []byte(`{"ref":"refs/heads/main"}`)

	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !handlers.VerifyGitHubSignature(secret, body, validSig) {
		t.Fatal("expected a correctly computed signature to verify")
	}
	if handlers.VerifyGitHubSignature(secret, body, "sha256=deadbeef") {
		t.Fatal("expected a wrong signature to fail verification")
	}
	if handlers.VerifyGitHubSignature(secret, body, validSig[len("sha256="):]) {
		t.Fatal("expected a signature missing the sha256= prefix to fail verification")
	}
	if handlers.VerifyGitHubSignature([]byte("wrong-secret"), body, validSig) {
		t.Fatal("expected a signature computed with a different secret to fail verification")
	}
}
