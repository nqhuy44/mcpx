package debug

import (
	"encoding/json"
	"regexp"
	"strings"
)

// LogError is an extracted error/fatal line from a log stream.
type LogError struct {
	LineNum int
	Level   string
	Message string
	Context []string // surrounding lines
}

var (
	levelRe      = regexp.MustCompile(`(?i)\b(ERROR|FATAL|PANIC|CRIT(?:ICAL)?|WARN(?:ING)?)\b`)
	jsonLineRe   = regexp.MustCompile(`^\s*\{`)
	structuredRe = regexp.MustCompile(`(?i)level[=:]["']?(error|fatal|panic|warn|warning|critical)["']?`)
)

func extractLogErrors(input string, contextLines int) []LogError {
	lines := strings.Split(input, "\n")
	var errors []LogError

	for i, line := range lines {
		level, msg := extractLevel(line)
		if level == "" || level == "WARN" {
			continue
		}

		start := i - contextLines
		if start < 0 {
			start = 0
		}
		end := i + contextLines + 1
		if end > len(lines) {
			end = len(lines)
		}

		var ctx []string
		for j := start; j < end; j++ {
			if j != i {
				ctx = append(ctx, strings.TrimRight(lines[j], "\r\n"))
			}
		}

		errors = append(errors, LogError{
			LineNum: i + 1,
			Level:   level,
			Message: msg,
			Context: ctx,
		})

		if len(errors) >= 10 {
			break
		}
	}

	return errors
}

func extractLevel(line string) (level, msg string) {
	if jsonLineRe.MatchString(line) {
		return extractJSONLevel(line)
	}
	if m := structuredRe.FindStringSubmatch(line); m != nil {
		return normalizeLevel(m[1]), extractStructuredMsg(line)
	}
	m := levelRe.FindStringSubmatch(line)
	if m == nil {
		return "", ""
	}
	upper := strings.ToUpper(line)
	idx := strings.Index(upper, strings.ToUpper(m[1]))
	if idx < 0 {
		return m[1], strings.TrimSpace(line)
	}
	msg = strings.TrimSpace(line[idx+len(m[1]):])
	msg = strings.TrimPrefix(msg, "]")
	msg = strings.TrimPrefix(msg, ":")
	msg = strings.TrimSpace(msg)
	return normalizeLevel(m[1]), msg
}

func extractJSONLevel(line string) (level, msg string) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return "", ""
	}
	for _, key := range []string{"level", "Level", "severity", "lvl"} {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok {
				level = normalizeLevel(s)
				break
			}
		}
	}
	if level == "" || (level != "ERROR" && level != "FATAL" && level != "PANIC") {
		return "", ""
	}
	for _, key := range []string{"msg", "message", "Message", "error", "err"} {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok {
				msg = s
				break
			}
		}
	}
	return level, msg
}

func extractStructuredMsg(line string) string {
	msgRe := regexp.MustCompile(`msg[=:]["']([^"']+)["']`)
	if m := msgRe.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return strings.TrimSpace(line)
}

func normalizeLevel(level string) string {
	switch strings.ToUpper(level) {
	case "FATAL", "CRIT", "CRITICAL":
		return "FATAL"
	case "PANIC":
		return "PANIC"
	case "WARNING":
		return "WARN"
	default:
		return strings.ToUpper(level)
	}
}
