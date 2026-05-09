package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	maxOutput  = 8 * 1024 // 8 KB per stream
	maxTimeout = 60
)

type Result struct {
	Stdout    string
	Stderr    string
	ExitCode  int
	Truncated bool
}

// RunCmd runs an arbitrary shell command in workdir.
// Used for project-level commands: go test ./..., make build, npm test, etc.
func RunCmd(cmd, workdir string, timeoutSecs int) (*Result, error) {
	if timeoutSecs <= 0 || timeoutSecs > maxTimeout {
		timeoutSecs = 30
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	if workdir != "" {
		c.Dir = workdir
	}

	var outBuf, errBuf bytes.Buffer
	c.Stdout = &outBuf
	c.Stderr = &errBuf

	runErr := c.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			if exitCode == -1 {
				exitCode = 1
			}
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = 124
		} else {
			return nil, runErr
		}
	}

	rawOut, rawErr := outBuf.String(), errBuf.String()
	truncated := len(rawOut) > maxOutput || len(rawErr) > maxOutput

	return &Result{
		Stdout:    truncate(rawOut, maxOutput),
		Stderr:    truncate(rawErr, maxOutput),
		ExitCode:  exitCode,
		Truncated: truncated,
	}, nil
}

// Run executes a code snippet in an isolated temp directory.
// Used for language snippets: python, javascript, bash, go, ruby, php.
func Run(lang, code, stdin string, timeoutSecs int) (*Result, error) {
	def, ok := langRegistry[strings.ToLower(lang)]
	if !ok {
		return nil, fmt.Errorf("unsupported language %q — supported: %s", lang, strings.Join(SupportedLangs(), ", "))
	}

	if timeoutSecs <= 0 || timeoutSecs > maxTimeout {
		timeoutSecs = 10
	}

	dir, err := os.MkdirTemp("", "mcpx-exec-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	scriptPath := filepath.Join(dir, "script"+def.ext)
	if err := os.WriteFile(scriptPath, []byte(code), 0600); err != nil {
		return nil, fmt.Errorf("write script: %w", err)
	}

	args := make([]string, len(def.cmd))
	for i, a := range def.cmd {
		if a == "SCRIPT" {
			args[i] = scriptPath
		} else {
			args[i] = a
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, args[0], args[1:]...)
	c.Dir = dir
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}

	var outBuf, errBuf bytes.Buffer
	c.Stdout = &outBuf
	c.Stderr = &errBuf

	runErr := c.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			if exitCode == -1 {
				exitCode = 1
			}
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = 124
		} else {
			return nil, runErr
		}
	}

	rawOut, rawErr := outBuf.String(), errBuf.String()
	truncated := len(rawOut) > maxOutput || len(rawErr) > maxOutput

	return &Result{
		Stdout:    truncate(rawOut, maxOutput),
		Stderr:    truncate(rawErr, maxOutput),
		ExitCode:  exitCode,
		Truncated: truncated,
	}, nil
}

// FilterTest extracts failing test information from test runner output.
// On success returns a short summary. On failure returns only the failing
// test names and their error messages — passing tests are dropped.
func FilterTest(r *Result) string {
	combined := r.Stdout
	if r.Stderr != "" {
		if combined != "" {
			combined += "\n"
		}
		combined += r.Stderr
	}

	passCount := strings.Count(combined, "--- PASS:") +
		strings.Count(combined, " ✓ ") +
		strings.Count(combined, " ✔ ")
	failCount := strings.Count(combined, "--- FAIL:") +
		strings.Count(combined, " ✕ ") +
		strings.Count(combined, " ✗ ") +
		strings.Count(combined, "FAILED")

	if r.ExitCode == 0 {
		if passCount > 0 {
			return fmt.Sprintf("All tests passed (%d)", passCount)
		}
		return "All tests passed"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "exit:%d  %d passed, %d failed\n", r.ExitCode, passCount, failCount)

	// Extract failure blocks: keep --- FAIL lines and their error context,
	// drop --- PASS lines and === RUN lines.
	lines := strings.Split(combined, "\n")
	inFail := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "=== RUN"), strings.HasPrefix(line, "=== PAUSE"), strings.HasPrefix(line, "=== CONT"):
			continue
		case strings.HasPrefix(line, "--- PASS:"):
			inFail = false
			continue
		case strings.HasPrefix(line, "--- FAIL:"):
			inFail = true
			sb.WriteByte('\n')
			sb.WriteString(line)
			sb.WriteByte('\n')
		case strings.Contains(line, "panic:"):
			inFail = true
			sb.WriteString(line)
			sb.WriteByte('\n')
		case inFail && strings.TrimSpace(line) != "":
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	out := strings.TrimRight(sb.String(), "\n")
	if len(out) > maxOutput {
		out = out[:maxOutput] + "\n... [truncated]"
	}
	return out
}

// FilterBuild extracts compiler errors from build output, dropping warnings
// and decoration lines. Falls back to raw capped output if no errors parsed.
func FilterBuild(r *Result) string {
	if r.ExitCode == 0 {
		return "Build succeeded"
	}

	// Prefer stderr for compiler output; fall back to stdout
	raw := r.Stderr
	if raw == "" {
		raw = r.Stdout
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "exit:%d\n\n", r.ExitCode)

	for _, line := range strings.Split(raw, "\n") {
		lower := strings.ToLower(line)
		// Drop pure decoration lines
		if strings.TrimSpace(line) == "" ||
			strings.Contains(lower, ": warning:") ||
			strings.Contains(lower, ": note:") ||
			strings.HasPrefix(strings.TrimSpace(line), "^") {
			continue
		}
		// Keep lines that look like errors
		if strings.Contains(lower, ": error:") ||
			strings.Contains(lower, "error[") ||
			strings.Contains(lower, "undefined:") ||
			strings.Contains(lower, "cannot ") ||
			strings.Contains(lower, "failed to") ||
			strings.HasPrefix(lower, "error:") {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	out := strings.TrimRight(sb.String(), "\n")
	// If nothing matched the error patterns, return raw (capped)
	if out == fmt.Sprintf("exit:%d", r.ExitCode) {
		return fmt.Sprintf("exit:%d\n%s", r.ExitCode, truncate(raw, maxOutput))
	}
	return out
}

// ── language snippet support ──────────────────────────────────────────────────

type langDef struct {
	ext string
	cmd []string // "SCRIPT" is replaced with the temp file path
}

var langRegistry = map[string]langDef{
	"python":     {".py", []string{"python3", "SCRIPT"}},
	"python3":    {".py", []string{"python3", "SCRIPT"}},
	"javascript": {".js", []string{"node", "SCRIPT"}},
	"js":         {".js", []string{"node", "SCRIPT"}},
	"node":       {".js", []string{"node", "SCRIPT"}},
	"bash":       {".sh", []string{"bash", "SCRIPT"}},
	"sh":         {".sh", []string{"sh", "SCRIPT"}},
	"go":         {".go", []string{"go", "run", "SCRIPT"}},
	"ruby":       {".rb", []string{"ruby", "SCRIPT"}},
	"php":        {".php", []string{"php", "SCRIPT"}},
}

func AvailableLangs() []string {
	seen := map[string]bool{}
	var out []string
	for name, def := range langRegistry {
		bin := def.cmd[0]
		if seen[bin] {
			continue
		}
		if _, err := exec.LookPath(bin); err == nil {
			seen[bin] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func SupportedLangs() []string {
	out := make([]string, 0, len(langRegistry))
	for k := range langRegistry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n... [%d bytes truncated]", len(s)-max)
}
