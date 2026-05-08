# mcpx-infra

Infrastructure tools for VMs and Kubernetes clusters. Queries Docker containers, systemd services, disk, processes, and Kubernetes resources — on the local machine, any SSH host from `~/.ssh/config`, or any Kubernetes context from `~/.kube/config`.

No extra configuration needed. Targets are auto-detected from your existing SSH and kubeconfig files.

## Tools

### Discovery

| Tool | Description |
|---|---|
| `infra_targets` | List all auto-detected targets — local, SSH hosts, Kubernetes contexts |

### VM / Docker (local or SSH target)

| Tool | Description |
|---|---|
| `infra_containers` | List Docker containers — status, image, ports. `all=true` includes stopped. |
| `infra_container_logs` | Container logs. `errors_only=true` filters to error/warning lines. |
| `infra_container_stats` | One-shot CPU and memory snapshot for all running containers. |
| `infra_compose` | List docker-compose stacks and their service status. |
| `infra_services` | List systemd services. `failed=true` shows only failed units. |
| `infra_service_logs` | Journald logs for a service. `errors_only=true` filters to error/warning lines. |
| `infra_disk` | Disk usage — real mount points sorted by usage percentage. |
| `infra_processes` | Top processes by CPU or memory. |

### Kubernetes (k8s target)

| Tool | Description |
|---|---|
| `infra_pods` | List pods — status, restarts, age, node. |
| `infra_pod_logs` | Pod logs. `errors_only=true` filters to error/warning lines. |
| `infra_deployments` | List deployments — ready/desired replicas. |
| `infra_k8s_events` | Warning events in a namespace, sorted by time. |

## Auto-detection

On startup the server discovers targets from existing config files:

| Source | Target type | Example |
|---|---|---|
| Always | `local` | local machine (default for VM tools) |
| `~/.ssh/config` | `ssh` | each `Host` block (wildcards skipped) |
| `~/.kube/config` | `kubernetes` | each context; `current-context` is default |

Run `infra_targets` to see everything that was detected:

```
NAME              TYPE        DEFAULT
local             local       yes
prod-vm           ssh         -
staging-vm        ssh         -
prod-cluster      kubernetes  yes
staging-cluster   kubernetes  -
```

## Usage

Every tool has an optional `target` parameter. Omit it to use the default.

```
# VM tools — default: local
infra_containers()                          → local docker ps
infra_containers(target="prod-vm")          → docker ps on prod-vm over SSH
infra_disk(target="staging-vm")             → df on staging-vm
infra_service_logs(target="prod-vm", name="api", errors_only=true)

# Kubernetes tools — default: current-context
infra_pods()                                → pods in current context
infra_pods(target="prod-cluster", namespace="payments")
infra_k8s_events(target="staging-cluster", namespace="default")
```

## SSH config example

Any host already in `~/.ssh/config` is available automatically:

```
Host prod-vm
    HostName 192.168.1.10
    User ubuntu
    IdentityFile ~/.ssh/prod_key

Host staging-vm
    HostName 10.0.0.5
    User ubuntu
```

The SSH key must not be passphrase-protected. If no `IdentityFile` is set, the server tries `~/.ssh/id_rsa`, `~/.ssh/id_ed25519`, and `~/.ssh/id_ecdsa` in order.

SSH connections are established lazily on first use and cached — one persistent connection per host, with automatic reconnect if stale.

## Kubernetes config

Any context in `~/.kube/config` (or `$KUBECONFIG`) is available automatically. Kubernetes tools run `kubectl --context=<name>` locally — no extra credentials needed beyond what is already in your kubeconfig.

## Gateway config

No `env` block required:

```yaml
# gateway.yaml
- name: infra
  transport: stdio
  binary: mcpx-infra
```

## Log filtering

`errors_only=true` on any log tool returns only lines matching:
`error`, `fatal`, `panic`, `exception`, `fail`, `critical`, `warn`

A 1000-line log typically yields 5–20 relevant lines — significant token savings when diagnosing failures.

## Prerequisites

| Feature | Requirement |
|---|---|
| Container tools | `docker` in PATH on the target |
| Compose listing | `docker compose` or `docker-compose` on the target |
| Service tools | `systemd` on the target (Linux only) |
| Kubernetes tools | `kubectl` in local PATH |
| SSH targets | Key-based auth configured in `~/.ssh/config` |

## Port (HTTP mode)

Default: `8088`. Override with `MCP_PORT` env var.
