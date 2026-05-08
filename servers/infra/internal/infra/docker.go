package infra

import (
	"context"
	"fmt"
	"strings"

	"github.com/nqhuy44/mcpx/servers/infra/internal/runner"
)

const maxLogBytes = 16 * 1024

// Containers lists containers. all=true includes stopped ones.
func Containers(ctx context.Context, run runner.RunFunc, all bool) (string, error) {
	flag := ""
	if all {
		flag = "-a"
	}
	out, err := run(ctx, fmt.Sprintf(
		`docker ps %s --format "{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"`, flag))
	if err != nil {
		return "", fmt.Errorf("docker ps: %w", err)
	}
	if out == "" {
		if all {
			return "no containers", nil
		}
		return "no running containers", nil
	}
	return "NAME\tIMAGE\tSTATUS\tPORTS\n" + out, nil
}

// ContainerLogs returns logs for a container, optionally filtered to errors only.
func ContainerLogs(ctx context.Context, run runner.RunFunc, name string, lines int, errorsOnly bool) (string, error) {
	if lines <= 0 {
		lines = 50
	}
	out, err := run(ctx, fmt.Sprintf("docker logs --tail %d %s 2>&1", lines, name))
	if err != nil {
		return "", fmt.Errorf("docker logs %s: %w", name, err)
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
		return "no output", nil
	}
	return out, nil
}

// ContainerStats returns a one-shot CPU/memory snapshot.
func ContainerStats(ctx context.Context, run runner.RunFunc) (string, error) {
	out, err := run(ctx,
		`docker stats --no-stream --format "{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}"`)
	if err != nil {
		return "", fmt.Errorf("docker stats: %w", err)
	}
	if out == "" {
		return "no running containers", nil
	}
	return "NAME\tCPU%\tMEM\tNET I/O\n" + out, nil
}

// Compose lists docker-compose stacks.
func Compose(ctx context.Context, run runner.RunFunc) (string, error) {
	out, err := run(ctx,
		`docker compose ls 2>/dev/null || docker-compose ls 2>/dev/null || echo "(compose not available)"`)
	if err != nil {
		return "(could not list compose stacks)", nil
	}
	lines := strings.Split(out, "\n")
	if len(lines) <= 1 {
		return "no compose stacks found", nil
	}
	// drop the header and re-add a clean one
	var result []string
	result = append(result, "NAME\tSTATUS\tCONFIG FILES")
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) != "" {
			result = append(result, l)
		}
	}
	return strings.Join(result, "\n"), nil
}
