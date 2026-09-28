// Package gitops fetches a manifest file out of a git repository and
// reports whether its content changed since the last check. It never
// decides how to apply the manifest — that's the caller's job (the
// handlers package), which knows whether to apply locally or relay to a
// remote node.
package gitops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"scutum/cmd/internal/utils"
)

// Result is what a fetch produced.
type Result struct {
	CommitSHA   string
	Content     []byte
	ContentHash string // sha256 hex of Content
	Changed     bool   // ContentHash differs from the previousContentHash passed in
}

// Fetch clones the repo into workDir if it doesn't exist yet, otherwise
// syncs it to the branch tip (fetch + hard-reset — see GitRepo.Sync), then
// reads manifestPath relative to the repo root. manifestPath is confined to
// workDir regardless of ".."/absolute-path content, the same way file
// distribution (#32) confines destination paths to its managed directory.
func Fetch(repo *utils.GitRepo, workDir, manifestPath, previousContentHash string) (Result, error) {
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		if err := repo.Clone(); err != nil {
			return Result{}, fmt.Errorf("clone: %w", err)
		}
	} else {
		if err := repo.Sync(); err != nil {
			return Result{}, fmt.Errorf("sync: %w", err)
		}
	}

	sha, err := repo.HeadSHA()
	if err != nil {
		return Result{}, fmt.Errorf("head sha: %w", err)
	}

	safePath := filepath.Join(workDir, filepath.Clean(string(filepath.Separator)+manifestPath))
	content, err := os.ReadFile(safePath)
	if err != nil {
		return Result{}, fmt.Errorf("read manifest %q: %w", manifestPath, err)
	}

	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])

	return Result{
		CommitSHA:   sha,
		Content:     content,
		ContentHash: hash,
		Changed:     hash != previousContentHash,
	}, nil
}
