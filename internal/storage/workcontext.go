package storage

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// WorkContext describes where a chat session was started (git repo or plain directory).
type WorkContext struct {
	WorkingDir string // absolute cwd when Dwight started
	GitRoot    string // absolute path to repo root, empty if not inside a git work tree
	OriginHint string // shortened origin URL (e.g. github.com/user/repo), empty if unavailable
}

// DetectWorkContext inspects cwd for a git repository and optional origin remote.
func DetectWorkContext(workingDir string) WorkContext {
	abs, err := filepath.Abs(workingDir)
	if err != nil {
		return WorkContext{}
	}
	wc := WorkContext{WorkingDir: abs}

	out, err := exec.Command("git", "-C", abs, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return wc
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return wc
	}
	wc.GitRoot = root

	urlOut, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output()
	if err != nil {
		return wc
	}
	wc.OriginHint = shortenGitRemote(strings.TrimSpace(string(urlOut)))
	return wc
}

var scpStyleRemote = regexp.MustCompile(`^[^@]+@([^:]+):(.+)$`)

// shortenGitRemote turns clone URLs into a short display label.
func shortenGitRemote(raw string) string {
	raw = strings.TrimSuffix(raw, ".git")
	raw = strings.TrimSuffix(raw, "/")

	if strings.HasPrefix(raw, "https://") {
		return strings.TrimPrefix(raw, "https://")
	}
	if strings.HasPrefix(raw, "http://") {
		return strings.TrimPrefix(raw, "http://")
	}
	if strings.HasPrefix(raw, "git@") {
		if m := scpStyleRemote.FindStringSubmatch(raw); len(m) == 3 {
			host := m[1]
			path := m[2]
			return host + "/" + path
		}
	}
	if i := strings.Index(raw, ":"); i > 0 && !strings.Contains(raw[:i], "/") {
		return raw[i+1:]
	}
	return raw
}

// ContextLabel returns a short string for lists: origin, repo folder name, or working dir.
func ContextLabel(wc WorkContext) string {
	if wc.OriginHint != "" {
		return wc.OriginHint
	}
	if wc.GitRoot != "" {
		return filepath.Base(wc.GitRoot)
	}
	if wc.WorkingDir != "" {
		return wc.WorkingDir
	}
	return ""
}
