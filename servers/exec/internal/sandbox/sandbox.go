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

type langDef struct {
	ext string
	cmd []string // "SCRIPT" is replaced with the temp file path
}

var registry = map[string]langDef{
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

func Run(lang, code, stdin string, timeoutSecs int) (*Result, error) {
	def, ok := registry[strings.ToLower(lang)]
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

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			if exitCode == -1 {
				exitCode = 1
			}
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = 124 // timeout (same as GNU timeout)
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

// AvailableLangs returns language names whose runtime is found in PATH.
func AvailableLangs() []string {
	seen := map[string]bool{}
	var out []string
	for name, def := range registry {
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

// SupportedLangs returns all registered language names regardless of availability.
func SupportedLangs() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
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
