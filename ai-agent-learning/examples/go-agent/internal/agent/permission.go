package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PermissionPolicy is intentionally conservative but educational. It is not a
// replacement for a container or an OS security boundary.
type PermissionPolicy struct {
	Workspace string
}

func (p PermissionPolicy) ResolvePath(path string) (string, error) {
	if p.Workspace == "" {
		return "", fmt.Errorf("permission policy has no workspace")
	}
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(p.Workspace, candidate)
	}
	resolved, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	workspace, err := filepath.Abs(p.Workspace)
	if err != nil {
		return "", err
	}
	cleanWorkspace := filepath.Clean(workspace)
	cleanResolved := filepath.Clean(resolved)
	if cleanResolved != cleanWorkspace && !strings.HasPrefix(cleanResolved, cleanWorkspace+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q is outside workspace", path)
	}
	return cleanResolved, nil
}

func (p PermissionPolicy) CheckCommand(command string) error {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return fmt.Errorf("command cannot be empty")
	}
	lower := strings.ToLower(trimmed)
	for _, denied := range []string{"rm -rf", "git push", "curl | sh", "wget | sh", ":(){:|:&};:"} {
		if strings.Contains(lower, denied) {
			return fmt.Errorf("command contains denied pattern %q", denied)
		}
	}
	return nil
}
