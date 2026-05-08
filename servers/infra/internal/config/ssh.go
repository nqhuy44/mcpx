package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// SSHHost represents a parsed Host block from ~/.ssh/config.
type SSHHost struct {
	Alias    string // first word after "Host"
	HostName string // defaults to Alias if not set
	User     string // empty = caller resolves to current user
	Port     string // defaults to "22"
	KeyFile  string // expanded path, empty = pool will try defaults
}

// ParseSSHConfig parses ~/.ssh/config and returns a list of concrete hosts.
// Wildcard entries (containing * or ?) are skipped.
// Returns nil, nil if the file does not exist.
func ParseSSHConfig() ([]SSHHost, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".ssh", "config")

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var hosts []SSHHost
	var current *SSHHost

	save := func() {
		if current == nil {
			return
		}
		if current.HostName == "" {
			current.HostName = current.Alias
		}
		if current.Port == "" {
			current.Port = "22"
		}
		hosts = append(hosts, *current)
		current = nil
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split into keyword and value (handle both "Key Value" and "Key=Value")
		line = strings.ReplaceAll(line, "=", " ")
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		keyword := strings.ToLower(parts[0])
		value := parts[1]

		if keyword == "host" {
			save()
			// Skip wildcard entries
			if strings.ContainsAny(value, "*?") {
				current = nil
				continue
			}
			current = &SSHHost{Alias: value}
			continue
		}

		if current == nil {
			continue
		}

		switch keyword {
		case "hostname":
			current.HostName = value
		case "user":
			current.User = value
		case "port":
			current.Port = value
		case "identityfile":
			current.KeyFile = expandHome(value, home)
		}
	}
	save()

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

func expandHome(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
