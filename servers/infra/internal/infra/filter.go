package infra

import "strings"

var errorKeywords = []string{"error", "fatal", "panic", "exception", "fail", "critical", "warn"}

// filterErrors returns only lines that contain error/warning keywords.
// Returns empty string if no matching lines found.
func filterErrors(logs string) string {
	var lines []string
	for _, line := range strings.Split(logs, "\n") {
		lower := strings.ToLower(line)
		for _, kw := range errorKeywords {
			if strings.Contains(lower, kw) {
				lines = append(lines, line)
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

// truncate caps output to maxBytes and appends a notice.
func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n... [output truncated]"
}
