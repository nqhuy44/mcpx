package debug

import (
	"fmt"
	"regexp"
	"strings"
)

const maxInputBytes = 512 * 1024

// Analyze takes raw text and returns a condensed, actionable analysis.
func Analyze(input, inputType, lang string, contextLines int) string {
	if len(input) > maxInputBytes {
		input = input[:maxInputBytes] + "\n[truncated]"
	}
	if inputType == "" || inputType == "auto" {
		inputType = detectType(input)
	}
	if lang == "" || lang == "auto" {
		lang = detectLang(input)
	}
	if contextLines <= 0 {
		contextLines = 3
	}

	r := &result{inputType: inputType, lang: lang}
	switch inputType {
	case "stacktrace", "panic":
		r.parseStacktrace(input)
	case "logs":
		r.parseLogs(input, contextLines)
	default:
		r.parseGeneric(input, contextLines)
	}
	r.suggestion = matchPattern(r.errorMsg)
	return r.format()
}

func detectType(input string) string {
	if strings.Contains(input, "panic:") ||
		strings.Contains(input, "Traceback (most recent call last)") ||
		strings.Contains(input, "goroutine ") ||
		strings.Contains(input, "stack backtrace:") ||
		strings.Contains(input, "Exception in thread") ||
		strings.Contains(input, "thread 'main' panicked") {
		return "panic"
	}
	logRe := regexp.MustCompile(`(?m)^\d{4}[-/]\d{2}[-/]\d{2}|^\{"level"`)
	if logRe.MatchString(input) ||
		strings.Contains(input, " ERROR ") ||
		strings.Contains(input, " FATAL ") ||
		strings.Contains(input, "[ERROR]") ||
		strings.Contains(input, "[FATAL]") {
		return "logs"
	}
	if regexp.MustCompile(`\.\w+:\d+`).MatchString(input) {
		return "stacktrace"
	}
	return "stderr"
}

func detectLang(input string) string {
	switch {
	case strings.Contains(input, "goroutine ") || strings.Contains(input, ".go:"):
		return "go"
	case strings.Contains(input, "Traceback (most recent call last)") || strings.Contains(input, ".py:"):
		return "python"
	case strings.Contains(input, "at Object.") || strings.Contains(input, "at Module.") ||
		strings.Contains(input, ".js:") || strings.Contains(input, ".ts:"):
		return "node"
	case strings.Contains(input, "Exception in thread") || strings.Contains(input, "at com.") ||
		strings.Contains(input, ".java:"):
		return "java"
	case strings.Contains(input, "thread 'main' panicked") || strings.Contains(input, ".rs:"):
		return "rust"
	}
	return "unknown"
}

type result struct {
	inputType  string
	lang       string
	errorMsg   string
	frames     []Frame
	logErrors  []LogError
	suggestion string
}

func (r *result) parseStacktrace(input string) {
	switch r.lang {
	case "go":
		r.errorMsg, r.frames = parseGo(input)
	case "python":
		r.errorMsg, r.frames = parsePython(input)
	case "node":
		r.errorMsg, r.frames = parseNode(input)
	case "java":
		r.errorMsg, r.frames = parseJava(input)
	case "rust":
		r.errorMsg, r.frames = parseRust(input)
	default:
		r.parseGeneric(input, 3)
	}
}

func (r *result) parseLogs(input string, contextLines int) {
	r.logErrors = extractLogErrors(input, contextLines)
	for _, e := range r.logErrors {
		if e.Level == "ERROR" || e.Level == "FATAL" || e.Level == "PANIC" {
			r.errorMsg = e.Message
			break
		}
	}
}

func (r *result) parseGeneric(input string, contextLines int) {
	r.logErrors = extractLogErrors(input, contextLines)
	if len(r.logErrors) > 0 {
		r.errorMsg = r.logErrors[0].Message
		return
	}
	for _, line := range strings.Split(input, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			r.errorMsg = line
			break
		}
	}
}

func (r *result) format() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "type:%s lang:%s\n\n", r.inputType, r.lang)

	if r.errorMsg != "" {
		fmt.Fprintf(&sb, "ERROR: %s\n", r.errorMsg)
	}

	if len(r.frames) > 0 {
		skipped := 0
		for _, f := range r.frames {
			if !f.IsUser {
				skipped++
				continue
			}
			if skipped > 0 {
				fmt.Fprintf(&sb, "  [%d stdlib/runtime frames]\n", skipped)
				skipped = 0
			}
			fmt.Fprintf(&sb, "  at %s:%d → %s\n", shortPath(f.File), f.Line, shortFunc(f.Function))
		}
		if skipped > 0 {
			fmt.Fprintf(&sb, "  [%d stdlib/runtime frames]\n", skipped)
		}
	}

	if len(r.logErrors) > 0 {
		errCount := 0
		for _, e := range r.logErrors {
			if e.Level != "WARN" {
				errCount++
			}
		}
		if errCount > 0 {
			fmt.Fprintf(&sb, "%d error(s) found\n\n", errCount)
		}
		for _, e := range r.logErrors {
			fmt.Fprintf(&sb, "[%d] %s: %s\n", e.LineNum, e.Level, e.Message)
			for _, ctx := range e.Context {
				fmt.Fprintf(&sb, "    %s\n", ctx)
			}
			sb.WriteByte('\n')
		}
	}

	if r.suggestion != "" {
		fmt.Fprintf(&sb, "CAUSE: %s\n", r.suggestion)
	}

	return strings.TrimRight(sb.String(), "\n")
}

func shortPath(file string) string {
	parts := strings.Split(strings.ReplaceAll(file, "\\", "/"), "/")
	if len(parts) > 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}
	return file
}

func shortFunc(fn string) string {
	if i := strings.Index(fn, "("); i > 0 {
		fn = fn[:i]
	}
	parts := strings.Split(fn, ".")
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return fn
}

var patterns = []struct {
	re  *regexp.Regexp
	fix string
}{
	{regexp.MustCompile(`(?i)connection refused`), "service not running or wrong host/port — check connectivity"},
	{regexp.MustCompile(`(?i)nil pointer dereference`), "nil pointer — add nil check before use"},
	{regexp.MustCompile(`(?i)index out of range`), "slice/array bounds exceeded — check length before indexing"},
	{regexp.MustCompile(`(?i)out of memory|cannot allocate`), "memory exhausted — check for leaks or increase limits"},
	{regexp.MustCompile(`(?i)permission denied`), "insufficient permissions — check file/socket ownership or user context"},
	{regexp.MustCompile(`(?i)no such file or directory`), "path does not exist — check workdir or config"},
	{regexp.MustCompile(`(?i)deadlock detected`), "goroutine deadlock — look for circular lock acquisition"},
	{regexp.MustCompile(`(?i)context deadline exceeded|context canceled`), "timeout or cancellation — check timeout values and upstream latency"},
	{regexp.MustCompile(`(?i)too many open files`), "fd limit reached — check ulimit or file handle leaks"},
	{regexp.MustCompile(`(?i)address already in use`), "port already bound — kill existing process or change port"},
	{regexp.MustCompile(`(?i)certificate|tls handshake`), "TLS/cert issue — check cert validity, CA trust, or hostname"},
	{regexp.MustCompile(`(?i)authentication failed|invalid credentials|unauthorized`), "auth failure — check credentials or token expiry"},
	{regexp.MustCompile(`(?i)SIGKILL`), "process killed by OS (likely OOM) — check memory limits"},
	{regexp.MustCompile(`(?i)stack overflow|maximum recursion`), "infinite recursion — check for unbounded recursive calls"},
	{regexp.MustCompile(`(?i)undefined.*reference|cannot find symbol|ImportError|ModuleNotFound`), "missing symbol/import — check dependencies or build tags"},
}

func matchPattern(msg string) string {
	for _, p := range patterns {
		if p.re.MatchString(msg) {
			return p.fix
		}
	}
	return ""
}
