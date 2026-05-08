Query infrastructure: Docker containers, systemd services, Kubernetes resources, disk, and processes.

User request: $ARGUMENTS

Use the infra_* MCP tools. Start with infra_targets if a target is not obvious.

## Routing by request type

**Containers / Docker:**
- "what's running" → infra_containers()
- "logs for X" → infra_container_logs(name="X", errors_only=true)
- "why is X crashing" → infra_container_logs(name="X", errors_only=true), then infra_container_stats() if OOM suspected
- "compose stacks" → infra_compose()

**Systemd services:**
- "what services are failing" → infra_services(failed=true)
- "logs for service X" → infra_service_logs(name="X", errors_only=true)
- "is nginx running" → infra_services(), filter output

**Kubernetes:**
- "pods in namespace X" → infra_pods(namespace="X")
- "why is pod X crashing" → infra_pod_logs(pod="X", errors_only=true)
- "deployment status" → infra_deployments()
- "k8s warnings" → infra_k8s_events()

**System health:**
- "disk usage" → infra_disk()
- "what's using CPU/memory" → infra_processes(sort_by="cpu") or sort_by="mem"

## Multi-target requests

If the user names a VM or cluster (e.g. "on prod-vm", "in prod-cluster"):
1. Call infra_targets() first to confirm the target name
2. Pass target= to every subsequent tool call

## Rules
- Always use errors_only=true on log tools unless the user explicitly wants full logs
- For "why is X down": check logs first, then stats — don't dump raw logs, explain the root cause
- Summarize table output — don't repeat every column if most are irrelevant to the question
- If a target is a kubernetes cluster and the user asks for containers, redirect to infra_pods
