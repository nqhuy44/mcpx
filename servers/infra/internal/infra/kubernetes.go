package infra

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/nqhuy44/mcpx/servers/infra/internal/runner"
)

// Pods lists pods in a given namespace (or all namespaces if namespace is empty).
func Pods(ctx context.Context, kubeCtx, namespace string) (string, error) {
	run := runner.LocalRunner()
	nsFlag := namespaceFlag(namespace)
	cmd := fmt.Sprintf("kubectl --context=%s get pods %s -o wide --no-headers 2>&1", kubeCtx, nsFlag)
	out, err := run(ctx, cmd)
	if err != nil {
		return "", kubectlErr(err)
	}
	if out == "" {
		return "no pods found", nil
	}
	return "NAMESPACE\tNAME\tREADY\tSTATUS\tRESTARTS\tAGE\tNODE\n" + truncate(out, maxLogBytes), nil
}

// PodLogs returns logs for a specific pod.
func PodLogs(ctx context.Context, kubeCtx, namespace, pod string, lines int, errorsOnly bool) (string, error) {
	if lines <= 0 {
		lines = 50
	}
	run := runner.LocalRunner()
	nsArg := ""
	if namespace != "" {
		nsArg = "-n " + namespace
	}
	cmd := fmt.Sprintf("kubectl --context=%s logs %s %s --tail=%d 2>&1", kubeCtx, nsArg, pod, lines)
	out, err := run(ctx, cmd)
	if err != nil {
		return "", kubectlErr(err)
	}
	out = truncate(out, maxLogBytes)
	if errorsOnly {
		filtered := filterErrors(out)
		if filtered == "" {
			return fmt.Sprintf("no error/warning lines in last %d lines of %s", lines, pod), nil
		}
		return filtered, nil
	}
	if out == "" {
		return "no logs", nil
	}
	return out, nil
}

// Deployments lists deployments in a given namespace (or all namespaces if namespace is empty).
func Deployments(ctx context.Context, kubeCtx, namespace string) (string, error) {
	run := runner.LocalRunner()
	nsFlag := namespaceFlag(namespace)
	cmd := fmt.Sprintf("kubectl --context=%s get deployments %s --no-headers 2>&1", kubeCtx, nsFlag)
	out, err := run(ctx, cmd)
	if err != nil {
		return "", kubectlErr(err)
	}
	if out == "" {
		return "no deployments found", nil
	}
	return "NAMESPACE\tNAME\tREADY\tUP-TO-DATE\tAVAILABLE\tAGE\n" + truncate(out, maxLogBytes), nil
}

// K8sEvents returns warning events sorted by timestamp.
func K8sEvents(ctx context.Context, kubeCtx, namespace string) (string, error) {
	run := runner.LocalRunner()
	nsFlag := namespaceFlag(namespace)
	cmd := fmt.Sprintf(
		"kubectl --context=%s get events %s --field-selector type=Warning --sort-by='.lastTimestamp' --no-headers 2>&1",
		kubeCtx, nsFlag)
	out, err := run(ctx, cmd)
	if err != nil {
		return "", kubectlErr(err)
	}
	if out == "" {
		return "no warning events", nil
	}
	return "NAMESPACE\tLAST SEEN\tTYPE\tREASON\tOBJECT\tMESSAGE\n" + truncate(out, maxLogBytes), nil
}

// namespaceFlag returns the appropriate kubectl namespace flag.
func namespaceFlag(namespace string) string {
	if namespace == "" {
		return "--all-namespaces"
	}
	return "-n " + namespace
}

// kubectlErr wraps errors with a hint when kubectl is not installed.
func kubectlErr(err error) error {
	if err == nil {
		return nil
	}
	if _, lookupErr := exec.LookPath("kubectl"); lookupErr != nil {
		return fmt.Errorf("kubectl not found in PATH — install kubectl to use kubernetes tools: %w", err)
	}
	msg := err.Error()
	// Strip redundant error prefix from combined output
	if idx := strings.Index(msg, "\n"); idx > 0 {
		return fmt.Errorf("%s", strings.TrimSpace(msg))
	}
	return err
}
