package utils

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type GitRepo struct {
	URL      string
	AuthUser string
	AuthPass string
	LocalDir string
	// Branch is only consulted by Clone/Sync (the GitOps reconciler path).
	// Empty means the repo's default branch.
	Branch string
}

func (g *GitRepo) authenticatedURL() string {
	const prefix = "https://"
	if strings.HasPrefix(g.URL, prefix) && g.AuthUser != "" && g.AuthPass != "" {
		return fmt.Sprintf("https://%s:%s@%s", g.AuthUser, g.AuthPass, g.URL[len(prefix):])
	}
	return g.URL
}

func (g *GitRepo) Clone() error {
	args := []string{"clone"}
	if g.Branch != "" {
		args = append(args, "-b", g.Branch, "--single-branch")
	}
	args = append(args, g.authenticatedURL(), g.LocalDir)

	cmd := exec.Command("git", args...)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Do not include command output — it may contain the authenticated URL.
		return errors.New("git clone failed: repository unreachable, branch not found, or credentials invalid")
	}
	return nil
}

// Pull updates an existing repository
func (g *GitRepo) Pull() error {
	cmd := exec.Command("git", "-C", g.LocalDir, "pull")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed: %s - %v", string(output), err)
	}
	return nil
}

// Sync brings LocalDir to exactly match the remote branch tip — fetch then
// hard-reset, rather than a merge-style Pull, so an automated reconciler
// never has to deal with local merge conflicts. Nothing is ever committed
// locally; this directory is a read-only mirror.
func (g *GitRepo) Sync() error {
	ref := g.Branch
	if ref == "" {
		ref = "HEAD"
	}
	if out, err := exec.Command("git", "-C", g.LocalDir, "fetch", "--depth", "1", "origin", ref).CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %s", string(out))
	}
	if out, err := exec.Command("git", "-C", g.LocalDir, "reset", "--hard", "FETCH_HEAD").CombinedOutput(); err != nil {
		return fmt.Errorf("git reset failed: %s", string(out))
	}
	return nil
}

// HeadSHA returns the current commit hash checked out at LocalDir.
func (g *GitRepo) HeadSHA() (string, error) {
	out, err := exec.Command("git", "-C", g.LocalDir, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %s", string(out))
	}
	return strings.TrimSpace(string(out)), nil
}
