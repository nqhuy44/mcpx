// Package gitops wraps local git CLI commands and returns token-lean output.
package gitops

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// run executes a git command in the given directory and returns trimmed stdout.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Status returns a compact working-tree status summary.
// Format: one line per changed file — "M path", "A path", "? path", etc.
func Status(repoDir string) (string, error) {
	out, err := run(repoDir, "status", "--short", "--branch")
	if err != nil {
		return "", err
	}
	if out == "" {
		return "clean", nil
	}
	return out, nil
}

// LogEntry is a single parsed commit.
type LogEntry struct {
	Hash    string
	Author  string
	Date    string
	Subject string
}

// Log returns recent commits filtered by optional author/path/since.
// n defaults to 20 if ≤0.
func Log(repoDir, author, path, since string, n int) ([]LogEntry, error) {
	if n <= 0 {
		n = 20
	}
	args := []string{
		"log",
		"--format=%H|%an|%ad|%s",
		"--date=short",
		"-n", strconv.Itoa(n),
	}
	if author != "" {
		args = append(args, "--author="+author)
	}
	if since != "" {
		args = append(args, "--since="+since)
	}
	if path != "" {
		args = append(args, "--", path)
	}

	out, err := run(repoDir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var entries []LogEntry
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}
		entries = append(entries, LogEntry{
			Hash:    parts[0][:min(8, len(parts[0]))],
			Author:  parts[1],
			Date:    parts[2],
			Subject: parts[3],
		})
	}
	return entries, nil
}

// Diff returns a diff for a file path, commit range, or staged changes.
// If ref is "staged" it diffs the index. If ref is empty it diffs the working tree.
func Diff(repoDir, ref, path string, stat bool) (string, error) {
	args := []string{"diff"}
	if stat {
		args = append(args, "--stat")
	}
	switch ref {
	case "staged":
		args = append(args, "--cached")
	case "":
		// working tree vs HEAD
	default:
		args = append(args, ref)
	}
	if path != "" {
		args = append(args, "--", path)
	}
	out, err := run(repoDir, args...)
	if err != nil {
		return "", err
	}
	if out == "" {
		return "no diff", nil
	}
	return out, nil
}

// BranchList returns all local branches with the current branch marked.
func BranchList(repoDir string) (string, error) {
	out, err := run(repoDir, "branch", "-v", "--no-color")
	if err != nil {
		return "", err
	}
	return out, nil
}

// BranchCreate creates a new branch from an optional base (defaults to HEAD).
func BranchCreate(repoDir, name, base string) (string, error) {
	args := []string{"checkout", "-b", name}
	if base != "" {
		args = append(args, base)
	}
	return run(repoDir, args...)
}

// BranchDelete deletes a local branch (force if force=true).
func BranchDelete(repoDir, name string, force bool) (string, error) {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return run(repoDir, "branch", flag, name)
}

// Blame returns annotated blame for a file, optionally limited to a line range.
// Lines are formatted as "hash date author: content".
func Blame(repoDir, path string, startLine, endLine int) (string, error) {
	args := []string{"blame", "--date=short", "-w"}
	if startLine > 0 && endLine >= startLine {
		args = append(args, fmt.Sprintf("-L%d,%d", startLine, endLine))
	}
	args = append(args, "--", path)
	out, err := run(repoDir, args...)
	if err != nil {
		return "", err
	}
	return out, nil
}

// Show returns a compact summary of a single commit: stat + message.
func Show(repoDir, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	out, err := run(repoDir, "show", "--stat", "--format=commit %h%nauthor %an%ndate %ad%n%n%s%n%b", "--date=short", ref)
	if err != nil {
		return "", err
	}
	return out, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
