package debug

import (
	"fmt"
	"regexp"
	"strings"
)

// Frame is a single stack frame.
type Frame struct {
	File     string
	Line     int
	Function string
	IsUser   bool // false = stdlib/runtime/vendor frame
}

// ── Go ───────────────────────────────────────────────────────────────────────

var (
	goFileLineRe = regexp.MustCompile(`^\s+(.+\.go):(\d+)`)
	goFuncRe     = regexp.MustCompile(`^(\S+)\(`)
)

func parseGo(input string) (errorMsg string, frames []Frame) {
	lines := strings.Split(input, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "panic: ") {
			errorMsg = strings.TrimPrefix(line, "panic: ")
			break
		}
		if strings.Contains(line, "runtime error:") {
			idx := strings.Index(line, "runtime error:")
			errorMsg = line[idx:]
			break
		}
		if strings.HasPrefix(line, "fatal error:") {
			errorMsg = line
			break
		}
	}

	for i := 0; i < len(lines)-1; i++ {
		funcMatch := goFuncRe.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if funcMatch == nil {
			continue
		}
		fileMatch := goFileLineRe.FindStringSubmatch(lines[i+1])
		if fileMatch == nil {
			continue
		}
		lineNum := 0
		fmt.Sscanf(fileMatch[2], "%d", &lineNum)
		frames = append(frames, Frame{
			File:     fileMatch[1],
			Line:     lineNum,
			Function: funcMatch[1],
			IsUser:   !isGoStdlib(funcMatch[1], fileMatch[1]),
		})
		i++
	}
	return errorMsg, frames
}

func isGoStdlib(funcName, file string) bool {
	for _, p := range []string{
		"runtime.", "sync.", "reflect.", "syscall.", "os.", "net.",
		"io.", "fmt.", "encoding/", "crypto/", "testing.", "http.",
		"bufio.", "bytes.", "strings.", "strconv.", "unicode.", "math.",
		"sort.", "time.", "context.", "errors.", "log.",
	} {
		if strings.HasPrefix(funcName, p) {
			return true
		}
	}
	return strings.Contains(file, "/usr/local/go/src/") ||
		strings.Contains(file, "GOROOT") ||
		strings.Contains(file, "/pkg/mod/")
}

// ── Python ───────────────────────────────────────────────────────────────────

var pyFileRe = regexp.MustCompile(`^\s+File "(.+)", line (\d+), in (.+)`)

func parsePython(input string) (errorMsg string, frames []Frame) {
	lines := strings.Split(input, "\n")
	inTraceback := false

	for i, line := range lines {
		if strings.TrimSpace(line) == "Traceback (most recent call last):" {
			inTraceback = true
			continue
		}
		if !inTraceback {
			continue
		}
		if m := pyFileRe.FindStringSubmatch(line); m != nil {
			lineNum := 0
			fmt.Sscanf(m[2], "%d", &lineNum)
			frames = append(frames, Frame{
				File:     m[1],
				Line:     lineNum,
				Function: m[3],
				IsUser:   !isPyStdlib(m[1]),
			})
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(line, " ") && i > 0 {
			errorMsg = trimmed
			inTraceback = false
		}
	}
	return errorMsg, frames
}

func isPyStdlib(file string) bool {
	return strings.Contains(file, "/lib/python") ||
		strings.Contains(file, "site-packages") ||
		strings.HasPrefix(file, "<frozen ")
}

// ── Node.js ──────────────────────────────────────────────────────────────────

var (
	nodeFrameRe     = regexp.MustCompile(`^\s+at (.+) \((.+):(\d+):\d+\)`)
	nodeFrameAnonRe = regexp.MustCompile(`^\s+at (.+):(\d+):\d+$`)
)

func parseNode(input string) (errorMsg string, frames []Frame) {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i == 0 || (len(frames) == 0 && !strings.HasPrefix(trimmed, "at ")) {
			if strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "at ") && trimmed != "" {
				errorMsg = trimmed
			}
			continue
		}
		if m := nodeFrameRe.FindStringSubmatch(line); m != nil {
			lineNum := 0
			fmt.Sscanf(m[3], "%d", &lineNum)
			frames = append(frames, Frame{
				File:     m[2],
				Line:     lineNum,
				Function: m[1],
				IsUser:   !isNodeStdlib(m[2]),
			})
			continue
		}
		if m := nodeFrameAnonRe.FindStringSubmatch(line); m != nil {
			lineNum := 0
			fmt.Sscanf(m[2], "%d", &lineNum)
			frames = append(frames, Frame{
				File:     m[1],
				Line:     lineNum,
				Function: "<anonymous>",
				IsUser:   !isNodeStdlib(m[1]),
			})
		}
	}
	return errorMsg, frames
}

func isNodeStdlib(file string) bool {
	return strings.HasPrefix(file, "node:") ||
		strings.Contains(file, "node_modules") ||
		strings.HasPrefix(file, "internal/") ||
		file == "native"
}

// ── Java ─────────────────────────────────────────────────────────────────────

var (
	javaFrameRe = regexp.MustCompile(`^\s+at ([\w.$]+)\((\S+):(\d+)\)`)
	javaErrorRe = regexp.MustCompile(`^(\w[\w.$]*(?:Exception|Error)): (.+)`)
)

func parseJava(input string) (errorMsg string, frames []Frame) {
	for _, line := range strings.Split(input, "\n") {
		if errorMsg == "" {
			if m := javaErrorRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
				errorMsg = m[1] + ": " + m[2]
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(line), "Exception in thread") {
				errorMsg = strings.TrimSpace(line)
				continue
			}
		}
		if m := javaFrameRe.FindStringSubmatch(line); m != nil {
			lineNum := 0
			fmt.Sscanf(m[3], "%d", &lineNum)
			frames = append(frames, Frame{
				File:     m[2],
				Line:     lineNum,
				Function: m[1],
				IsUser:   !isJavaStdlib(m[1]),
			})
		}
	}
	return errorMsg, frames
}

func isJavaStdlib(className string) bool {
	for _, p := range []string{"java.", "javax.", "sun.", "com.sun.", "org.springframework.", "io.netty."} {
		if strings.HasPrefix(className, p) {
			return true
		}
	}
	return false
}

// ── Rust ─────────────────────────────────────────────────────────────────────

var (
	rustPanicRe = regexp.MustCompile(`thread '.+' panicked at '(.+)',`)
	rustFrameRe = regexp.MustCompile(`^\s+\d+:\s+(.+)`)
	rustFileRe  = regexp.MustCompile(`^\s+at (.+):(\d+)`)
)

func parseRust(input string) (errorMsg string, frames []Frame) {
	lines := strings.Split(input, "\n")
	var pendingFunc string

	for _, line := range lines {
		if errorMsg == "" {
			if m := rustPanicRe.FindStringSubmatch(line); m != nil {
				errorMsg = "panic: " + m[1]
				continue
			}
		}
		if m := rustFrameRe.FindStringSubmatch(line); m != nil {
			pendingFunc = strings.TrimSpace(m[1])
			continue
		}
		if m := rustFileRe.FindStringSubmatch(line); m != nil && pendingFunc != "" {
			lineNum := 0
			fmt.Sscanf(m[2], "%d", &lineNum)
			frames = append(frames, Frame{
				File:     m[1],
				Line:     lineNum,
				Function: pendingFunc,
				IsUser:   !isRustStdlib(m[1], pendingFunc),
			})
			pendingFunc = ""
		}
	}
	return errorMsg, frames
}

func isRustStdlib(file, fn string) bool {
	return strings.Contains(file, "/.rustup/") ||
		strings.Contains(file, "/registry/src/") ||
		strings.HasPrefix(fn, "std::") ||
		strings.HasPrefix(fn, "core::") ||
		strings.HasPrefix(fn, "alloc::") ||
		strings.HasPrefix(fn, "<std::") ||
		strings.HasPrefix(fn, "<alloc::")
}
