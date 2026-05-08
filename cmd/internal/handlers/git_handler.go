package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"scutum/cmd/internal/utils"
)

type GitSyncRequest struct {
	RepoURL  string `json:"repo_url"`
	Username string `json:"username"`
	Token    string `json:"token"`
	Target   string `json:"target_dir"`
}

type GitHandler struct {
	validateURL func(string) bool
}

// WithURLValidator overrides the URL scheme check — use in tests only.
func WithURLValidator(fn func(string) bool) func(*GitHandler) {
	return func(h *GitHandler) { h.validateURL = fn }
}

func NewGitHandler(opts ...func(*GitHandler)) *GitHandler {
	h := &GitHandler{validateURL: func(url string) bool {
		return strings.HasPrefix(url, "https://")
	}}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *GitHandler) HandleGitSync(w http.ResponseWriter, r *http.Request) {
	var req GitSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !h.validateURL(req.RepoURL) {
		http.Error(w, "only https:// repository URLs are supported", http.StatusBadRequest)
		return
	}

	baseDir := os.Getenv("SCUTUM_STACKS_DIR")
	if baseDir == "" {
		baseDir = "/var/lib/scutum/stacks"
	}

	localDir := filepath.Join(baseDir, req.Target)
	if !strings.HasPrefix(localDir, filepath.Clean(baseDir)+string(filepath.Separator)) {
		http.Error(w, "invalid target directory", http.StatusBadRequest)
		return
	}

	repo := utils.GitRepo{
		URL:      req.RepoURL,
		AuthUser: req.Username,
		AuthPass: req.Token,
		LocalDir: localDir,
	}

	// Check if repo already exists
	if _, err := os.Stat(repo.LocalDir); os.IsNotExist(err) {
		if err := repo.Clone(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if err := repo.Pull(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Sync successful"))
}
