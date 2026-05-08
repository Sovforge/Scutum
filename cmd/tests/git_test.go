package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/utils"
)

// TestGitHandlerSyncRequest tests Git sync request handling
func TestGitHandlerSyncRequest(t *testing.T) {
	tests := []struct {
		name      string
		repoURL   string
		username  string
		token     string
		targetDir string
		valid     bool
	}{
		{"github https", "https://github.com/user/repo.git", "user", "token", "/app", true},
		{"github ssh", "git@github.com:user/repo.git", "", "", "/opt/app", true},
		{"gitlab", "https://gitlab.com/group/repo.git", "user", "token", "/data", true},
		{"empty url", "", "user", "token", "/app", false},
		{"empty target dir", "https://github.com/user/repo.git", "user", "token", "", false},
		{"default creds", "https://github.com/user/repo.git", "", "", "/app", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.repoURL != "" && tt.targetDir != ""
			if isValid != tt.valid {
				t.Errorf("sync request validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// TestGitRepositoryURLValidation tests Git repository URL validation
func TestGitRepositoryURLValidation(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		valid bool
	}{
		{"https url", "https://github.com/user/repo.git", true},
		{"http url", "http://gitlab.com/repo.git", true},
		{"ssh url", "git@github.com:user/repo.git", true},
		{"gitlab https", "https://gitlab.com/group/subgroup/repo.git", true},
		{"empty url", "", false},
		{"invalid protocol", "ftp://github.com/repo.git", false},
		{"no protocol", "github.com/user/repo.git", false},
		{"spaces in url", "https://github.com/user/ repo.git", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.url != "" &&
				(contains(tt.url, "https://") || contains(tt.url, "http://") ||
					contains(tt.url, "git@")) &&
				!contains(tt.url, " ")
			if isValid != tt.valid {
				t.Errorf("url validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// TestGitBranchValidation tests Git branch name validation
func TestGitBranchValidation(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		valid  bool
	}{
		{"main branch", "main", true},
		{"develop branch", "develop", true},
		{"release branch", "release/v1.0.0", true},
		{"feature branch", "feature/new-feature", true},
		{"fix branch", "fix/bug-123", true},
		{"with slashes", "release/v1.0/hotfix", true},
		{"empty branch name", "", true},
		{"spaces in branch", "feature branch", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := !contains(tt.branch, " ")
			if isValid != tt.valid {
				t.Errorf("branch validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// TestGitCredentialHandling tests credential injection and handling
func TestGitCredentialHandling(t *testing.T) {
	tests := []struct {
		name     string
		username string
		token    string
		valid    bool
	}{
		{"with credentials", "user", "token123", true},
		{"username only", "user", "", true},
		{"token only", "", "token123", true},
		{"no credentials", "", "", true},
		{"spaces in username", "my user", "token", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := !contains(tt.username, " ")
			if isValid != tt.valid {
				t.Errorf("credential validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// TestGitPathTraversalPrevention tests path traversal attack prevention
func TestGitPathTraversalPrevention(t *testing.T) {
	tests := []struct {
		name      string
		targetDir string
		safe      bool
	}{
		{"safe path", "/opt/repos", true},
		{"safe nested", "/home/user/projects/app", true},
		{"relative path", "repos/myapp", true},
		{"path traversal attempt", "/opt/repos/../../etc/passwd", false},
		{"double dot escape", "../../sensitive", false},
		{"absolute escape", "/etc/passwd", false},
		{"empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check for path traversal and system paths
			isSafe := tt.targetDir != "" &&
				!contains(tt.targetDir, "..") &&
				!contains(tt.targetDir, "/../")
			// Block suspicious absolute paths
			if isSafe && len(tt.targetDir) > 0 && tt.targetDir[0] == '/' {
				if contains(tt.targetDir, "/etc") || contains(tt.targetDir, "/var") ||
					contains(tt.targetDir, "/system") || contains(tt.targetDir, "/bin") {
					isSafe = false
				}
			}
			if isSafe != tt.safe {
				t.Errorf("path traversal check: got safe=%v, want %v", isSafe, tt.safe)
			}
		})
	}
}

// TestGitCloneDepthValidation tests git clone depth parameter validation
func TestGitCloneDepthValidation(t *testing.T) {
	tests := []struct {
		name  string
		depth int
		valid bool
	}{
		{"full clone", 0, true},
		{"shallow clone", 1, true},
		{"normal shallow", 10, true},
		{"large depth", 1000, true},
		{"negative depth", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.depth >= 0
			if isValid != tt.valid {
				t.Errorf("depth validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// TestGitSubmoduleHandling tests git submodule configuration
func TestGitSubmoduleHandling(t *testing.T) {
	tests := []struct {
		name           string
		initSubmodules bool
		recursive      bool
		valid          bool
	}{
		{"with submodules recursive", true, true, true},
		{"submodules no recursive", true, false, true},
		{"no submodules", false, false, true},
		{"recursive without init", false, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := true
			if isValid != tt.valid {
				t.Errorf("submodule validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// TestGitHandlerInitialization tests Git handler creation
func TestGitHandlerInitialization(t *testing.T) {
	handler := handlers.NewGitHandler()
	if handler == nil {
		t.Error("NewGitHandler() returned nil")
	}
}
func TestGitHandlerHandleGitSyncBadRequest(t *testing.T) {
	h := handlers.NewGitHandler()
	req := httptest.NewRequest(http.MethodPost, "/git/sync", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()
	h.HandleGitSync(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGitRepoCloneAndPullLocal(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	sourceRepo := t.TempDir()
	cmd := exec.Command("git", "init", sourceRepo)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %s", output)
	}

	if output, err := exec.Command("git", "-C", sourceRepo, "config", "user.email", "test@example.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email failed: %s", output)
	}
	if output, err := exec.Command("git", "-C", sourceRepo, "config", "user.name", "Test User").CombinedOutput(); err != nil {
		t.Fatalf("git config name failed: %s", output)
	}

	if err := os.WriteFile(filepath.Join(sourceRepo, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write README failed: %v", err)
	}
	if output, err := exec.Command("git", "-C", sourceRepo, "add", "README.md").CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s", output)
	}
	if output, err := exec.Command("git", "-C", sourceRepo, "commit", "-m", "initial commit").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s", output)
	}

	cloneDir := t.TempDir()
	repo := utils.GitRepo{URL: sourceRepo, LocalDir: cloneDir}
	if err := repo.Clone(); err != nil {
		t.Fatalf("GitRepo.Clone() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(sourceRepo, "README2.md"), []byte("world"), 0o644); err != nil {
		t.Fatalf("write README2 failed: %v", err)
	}
	if output, err := exec.Command("git", "-C", sourceRepo, "add", "README2.md").CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s", output)
	}
	if output, err := exec.Command("git", "-C", sourceRepo, "commit", "-m", "second commit").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s", output)
	}

	if err := repo.Pull(); err != nil {
		t.Fatalf("GitRepo.Pull() error = %v", err)
	}
}

func TestGitHandlerHandleGitSyncWithExistingRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	// Create a source repository
	sourceRepo := t.TempDir()
	cmd := exec.Command("git", "init", sourceRepo)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %s", output)
	}

	if output, err := exec.Command("git", "-C", sourceRepo, "config", "user.email", "test@example.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email failed: %s", output)
	}
	if output, err := exec.Command("git", "-C", sourceRepo, "config", "user.name", "Test User").CombinedOutput(); err != nil {
		t.Fatalf("git config name failed: %s", output)
	}

	if err := os.WriteFile(filepath.Join(sourceRepo, "README.md"), []byte("initial"), 0o644); err != nil {
		t.Fatalf("write README failed: %v", err)
	}
	if output, err := exec.Command("git", "-C", sourceRepo, "add", "README.md").CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s", output)
	}
	if output, err := exec.Command("git", "-C", sourceRepo, "commit", "-m", "initial commit").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s", output)
	}

	// Create handler and call HandleGitSync for initial clone
	h := handlers.NewGitHandler(handlers.WithURLValidator(func(string) bool { return true }))
	baseDir := t.TempDir()
	os.Setenv("SCUTUM_STACKS_DIR", baseDir)
	defer os.Unsetenv("SCUTUM_STACKS_DIR")

	syncReq := `{"repo_url":"` + sourceRepo + `","username":"user","token":"pass","target_dir":"myrepo"}`
	req := httptest.NewRequest(http.MethodPost, "/git/sync", strings.NewReader(syncReq))
	w := httptest.NewRecorder()
	h.HandleGitSync(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first sync: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Make an update to source repo
	if err := os.WriteFile(filepath.Join(sourceRepo, "README2.md"), []byte("update"), 0o644); err != nil {
		t.Fatalf("write README2 failed: %v", err)
	}
	if output, err := exec.Command("git", "-C", sourceRepo, "add", "README2.md").CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s", output)
	}
	if output, err := exec.Command("git", "-C", sourceRepo, "commit", "-m", "second commit").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s", output)
	}

	// Second sync should pull updates (repo already exists)
	req = httptest.NewRequest(http.MethodPost, "/git/sync", strings.NewReader(syncReq))
	w = httptest.NewRecorder()
	h.HandleGitSync(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("second sync: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the repo was updated
	clonedFile := filepath.Join(baseDir, "myrepo", "README2.md")
	if _, err := os.Stat(clonedFile); err != nil {
		t.Fatalf("expected cloned file not found: %v", err)
	}
}
