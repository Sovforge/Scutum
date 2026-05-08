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
}

func (g *GitRepo) Clone() error {
	const prefix = "https://"
	authenticatedURL := g.URL
	if strings.HasPrefix(g.URL, prefix) && g.AuthUser != "" && g.AuthPass != "" {
		authenticatedURL = fmt.Sprintf("https://%s:%s@%s",
			g.AuthUser, g.AuthPass, g.URL[len(prefix):])
	}

	cmd := exec.Command("git", "clone", authenticatedURL, g.LocalDir)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Do not include command output — it may contain the authenticated URL.
		return errors.New("git clone failed: repository unreachable or credentials invalid")
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
