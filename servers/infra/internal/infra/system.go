package infra

import (
	"context"
	"fmt"
	"strings"

	"github.com/nqhuy44/mcpx/servers/infra/internal/runner"
)

// Services lists systemd services. failed=true shows only failed units.
func Services(ctx context.Context, run runner.RunFunc, failed bool) (string, error) {
	filter := ""
	if failed {
		filter = "--state=failed"
	}
	out, err := run(ctx, fmt.Sprintf(
		`systemctl list-units %s --type=service --no-pager --no-legend --plain 2>/dev/null | awk '{print $1"\t"$3"\t"$4"\t"$5}'`,
		filter))
	if err != nil {
		return "", fmt.Errorf("systemctl: %w", err)
	}
	if out == "" {
		if failed {
			return "no failed services", nil
		}
		return "no services found", nil
	}
	return "SERVICE\tLOAD\tACTIVE\tSUB\n" + out, nil
}

// ServiceLogs returns journald logs for a service.
func ServiceLogs(ctx context.Context, run runner.RunFunc, name string, lines int, errorsOnly bool) (string, error) {
	if lines <= 0 {
		lines = 50
	}
	out, err := run(ctx, fmt.Sprintf(
		"journalctl -u %s -n %d --no-pager --output=short 2>&1", name, lines))
	if err != nil {
		return "", fmt.Errorf("journalctl -u %s: %w", name, err)
	}
	out = truncate(out, maxLogBytes)
	if errorsOnly {
		filtered := filterErrors(out)
		if filtered == "" {
			return fmt.Sprintf("no error/warning lines in last %d lines of %s", lines, name), nil
		}
		return filtered, nil
	}
	if out == "" {
		return "no logs", nil
	}
	return out, nil
}

// Disk returns disk usage for real filesystems, sorted by usage %.
func Disk(ctx context.Context, run runner.RunFunc) (string, error) {
	out, err := run(ctx,
		`df -h --output=target,size,used,avail,pcent 2>/dev/null | grep -Ev '^(Filesystem|tmpfs|udev|devtmpfs|overlay|shm)' | sort -k5 -rn | head -15`)
	if err != nil {
		return "", fmt.Errorf("df: %w", err)
	}
	if out == "" {
		return "no filesystem data", nil
	}
	return "MOUNT\tSIZE\tUSED\tAVAIL\tUSE%\n" + out, nil
}

// TopProcesses returns top N processes sorted by cpu or mem.
func TopProcesses(ctx context.Context, run runner.RunFunc, sortBy string, n int) (string, error) {
	if n <= 0 || n > 50 {
		n = 10
	}
	col := "3" // default: CPU%
	if strings.HasPrefix(strings.ToLower(sortBy), "mem") {
		col = "4"
	}
	// ps aux columns: USER PID CPU% MEM% VSZ RSS TTY STAT START TIME CMD
	out, err := run(ctx, fmt.Sprintf(
		`ps aux --sort=-%s 2>/dev/null | awk 'NR>1 {printf "%%s\t%%s%%\t%%s%%\t%%s\n",$11,$3,$4,$1}' | head -n %d`,
		col, n))
	if err != nil {
		return "", fmt.Errorf("ps: %w", err)
	}
	if out == "" {
		return "no processes", nil
	}
	return "CMD\tCPU%\tMEM%\tUSER\n" + out, nil
}
