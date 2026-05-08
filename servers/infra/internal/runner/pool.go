package runner

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nqhuy44/mcpx/servers/infra/internal/config"
	"golang.org/x/crypto/ssh"
)

// TargetType identifies how a target is reached.
type TargetType string

const (
	TypeLocal      TargetType = "local"
	TypeSSH        TargetType = "ssh"
	TypeKubernetes TargetType = "kubernetes"
)

// Target is a named execution endpoint.
type Target struct {
	Name    string
	Type    TargetType
	Default bool
	// SSH fields
	Host    string
	User    string
	Port    string
	KeyFile string
	// Kubernetes fields
	Context string
}

// RunFunc executes a shell command and returns combined output.
type RunFunc func(ctx context.Context, cmd string) (string, error)

// Pool manages a set of named targets and their SSH connections.
type Pool struct {
	targets  map[string]*Target
	sshConns map[string]*ssh.Client
	mu       sync.Mutex
}

// NewPool builds a Pool from local machine, ~/.ssh/config, and ~/.kube/config.
func NewPool() (*Pool, error) {
	p := &Pool{
		targets:  make(map[string]*Target),
		sshConns: make(map[string]*ssh.Client),
	}

	// Always add local target as default VM target.
	p.targets["local"] = &Target{
		Name:    "local",
		Type:    TypeLocal,
		Default: true,
	}

	// Parse SSH hosts.
	sshHosts, err := config.ParseSSHConfig()
	if err != nil {
		return nil, fmt.Errorf("parse ssh config: %w", err)
	}
	currentUser := ""
	if u, err := user.Current(); err == nil {
		currentUser = u.Username
	}
	for _, h := range sshHosts {
		u := h.User
		if u == "" {
			u = currentUser
		}
		port := h.Port
		if port == "" {
			port = "22"
		}
		hostname := h.HostName
		if hostname == "" {
			hostname = h.Alias
		}
		p.targets[h.Alias] = &Target{
			Name:    h.Alias,
			Type:    TypeSSH,
			Default: false,
			Host:    hostname,
			User:    u,
			Port:    port,
			KeyFile: h.KeyFile,
		}
	}

	// Parse Kubernetes contexts.
	kubeCfg, err := config.ParseKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("parse kube config: %w", err)
	}
	for _, ctxName := range kubeCfg.Contexts {
		name := ctxName
		if _, exists := p.targets[name]; exists {
			// Collision with SSH host — suffix k8s name.
			name = ctxName + "-k8s"
		}
		p.targets[name] = &Target{
			Name:    name,
			Type:    TypeKubernetes,
			Default: ctxName == kubeCfg.CurrentContext,
			Context: ctxName,
		}
	}

	return p, nil
}

// All returns all targets: local first, then SSH alphabetically, then k8s alphabetically.
func (p *Pool) All() []*Target {
	p.mu.Lock()
	defer p.mu.Unlock()

	var local, sshTargets, k8sTargets []*Target
	for _, t := range p.targets {
		switch t.Type {
		case TypeLocal:
			local = append(local, t)
		case TypeSSH:
			sshTargets = append(sshTargets, t)
		case TypeKubernetes:
			k8sTargets = append(k8sTargets, t)
		}
	}
	sort.Slice(sshTargets, func(i, j int) bool { return sshTargets[i].Name < sshTargets[j].Name })
	sort.Slice(k8sTargets, func(i, j int) bool { return k8sTargets[i].Name < k8sTargets[j].Name })

	result := make([]*Target, 0, len(p.targets))
	result = append(result, local...)
	result = append(result, sshTargets...)
	result = append(result, k8sTargets...)
	return result
}

// VMRunner returns a RunFunc for the named target.
// An empty name resolves to the local target.
func (p *Pool) VMRunner(name string) (RunFunc, error) {
	if name == "" {
		return localRunFunc(), nil
	}
	p.mu.Lock()
	t, ok := p.targets[name]
	p.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("unknown target %q — run infra_targets to list available", name)
	}
	switch t.Type {
	case TypeKubernetes:
		return nil, fmt.Errorf("target %q is a kubernetes cluster — use infra_pods / infra_pod_logs / infra_deployments / infra_k8s_events", name)
	case TypeLocal:
		return localRunFunc(), nil
	case TypeSSH:
		return p.sshRunFunc(t), nil
	}
	return nil, fmt.Errorf("unknown target type for %q", name)
}

// KubeContext returns the kubectl context name for the named target.
// An empty name resolves to the current-context target.
func (p *Pool) KubeContext(name string) (string, error) {
	if name == "" {
		p.mu.Lock()
		defer p.mu.Unlock()
		for _, t := range p.targets {
			if t.Default && t.Type == TypeKubernetes {
				return t.Context, nil
			}
		}
		return "", fmt.Errorf("no kubernetes target found — check ~/.kube/config")
	}
	p.mu.Lock()
	t, ok := p.targets[name]
	p.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("unknown target %q — run infra_targets to list available", name)
	}
	if t.Type == TypeSSH || t.Type == TypeLocal {
		return "", fmt.Errorf("%q is not a kubernetes target", name)
	}
	return t.Context, nil
}

// Close shuts down all cached SSH connections.
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.sshConns {
		c.Close()
	}
}

// localRunFunc returns a RunFunc that executes commands on the local machine.
func localRunFunc() RunFunc {
	return func(ctx context.Context, cmd string) (string, error) {
		c := exec.CommandContext(ctx, "sh", "-c", cmd)
		out, err := c.CombinedOutput()
		result := strings.TrimSpace(string(out))
		if err != nil {
			return result, fmt.Errorf("%w\n%s", err, result)
		}
		return result, nil
	}
}

// LocalRunner returns a standalone RunFunc for local execution (used by kubernetes.go).
func LocalRunner() RunFunc {
	return localRunFunc()
}

// sshRunFunc returns a RunFunc that executes commands over SSH (lazy connect, cached).
func (p *Pool) sshRunFunc(t *Target) RunFunc {
	return func(ctx context.Context, cmd string) (string, error) {
		conn, err := p.getSSHConn(t)
		if err != nil {
			return "", err
		}
		return runSSH(ctx, conn, cmd)
	}
}

// getSSHConn returns a cached (or newly dialed) SSH client for the target.
func (p *Pool) getSSHConn(t *Target) (*ssh.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	conn, exists := p.sshConns[t.Name]
	if exists {
		// Send keepalive; reconnect if the connection is stale.
		_, _, err := conn.SendRequest("keepalive@openssh.com", true, nil)
		if err != nil {
			conn.Close()
			delete(p.sshConns, t.Name)
			exists = false
		}
	}
	if !exists {
		var err error
		conn, err = dialSSH(t)
		if err != nil {
			return nil, err
		}
		p.sshConns[t.Name] = conn
	}
	return conn, nil
}

// dialSSH establishes an SSH connection to the target.
func dialSSH(t *Target) (*ssh.Client, error) {
	signer, err := loadSigner(t.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("ssh %s: load key: %w", t.Name, err)
	}

	cfg := &ssh.ClientConfig{
		User:            t.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         10 * time.Second,
	}

	conn, err := ssh.Dial("tcp", net.JoinHostPort(t.Host, t.Port), cfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s@%s:%s: %w", t.User, t.Host, t.Port, err)
	}
	return conn, nil
}

// loadSigner attempts to load a private key. If keyFile is empty, tries well-known defaults.
func loadSigner(keyFile string) (ssh.Signer, error) {
	home, _ := os.UserHomeDir()

	candidates := []string{keyFile}
	if keyFile == "" {
		candidates = []string{
			filepath.Join(home, ".ssh", "id_rsa"),
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_ecdsa"),
		}
	}

	var lastErr error
	for _, path := range candidates {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			lastErr = fmt.Errorf("parse key %s: %w", path, err)
			continue
		}
		return signer, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no SSH key found (tried id_rsa, id_ed25519, id_ecdsa)")
}

// runSSH opens a new session on the given client and runs cmd.
func runSSH(ctx context.Context, conn *ssh.Client, cmd string) (string, error) {
	sess, err := conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}
	defer sess.Close()

	var buf bytes.Buffer
	sess.Stdout = &buf
	sess.Stderr = &buf

	done := make(chan error, 1)
	go func() { done <- sess.Run(cmd) }()

	select {
	case <-ctx.Done():
		sess.Signal(ssh.SIGTERM) //nolint:errcheck
		return "", ctx.Err()
	case err := <-done:
		out := strings.TrimSpace(buf.String())
		if err != nil {
			return out, fmt.Errorf("%w\n%s", err, out)
		}
		return out, nil
	}
}
