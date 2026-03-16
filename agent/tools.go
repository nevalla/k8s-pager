package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nevalla/k8s-pager/kube"
)

func DefaultTools() []Tool {
	return []Tool{
		{
			Name:        "describe_pod",
			Description: "Get detailed status of a Kubernetes pod including conditions, container statuses, and events.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"namespace": map[string]string{"type": "string", "description": "Pod namespace"},
					"name":      map[string]string{"type": "string", "description": "Pod name"},
				},
				"required": []string{"namespace", "name"},
			},
		},
		{
			Name:        "get_pod_logs",
			Description: "Get recent log lines from a pod's containers.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"namespace":  map[string]string{"type": "string", "description": "Pod namespace"},
					"name":       map[string]string{"type": "string", "description": "Pod name"},
					"tail_lines": map[string]any{"type": "integer", "description": "Number of lines from the end", "default": 100},
				},
				"required": []string{"namespace", "name"},
			},
		},
		{
			Name:        "get_events",
			Description: "Get Kubernetes events related to a specific resource.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"namespace": map[string]string{"type": "string", "description": "Resource namespace"},
					"name":      map[string]string{"type": "string", "description": "Resource name"},
				},
				"required": []string{"namespace", "name"},
			},
		},
		{
			Name:        "get_node_status",
			Description: "Get status and conditions of a Kubernetes node.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"node_name": map[string]string{"type": "string", "description": "Node name"},
				},
				"required": []string{"node_name"},
			},
		},
		{
			Name:        "get_resource",
			Description: "Get status of a Kubernetes custom resource (e.g., HelmRelease, Kustomization, GitRepository, HelmChart, HelmRepository, OCIRepository, Bucket).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"kind":      map[string]string{"type": "string", "description": "Resource kind (e.g., HelmRelease, Kustomization, GitRepository)"},
					"namespace": map[string]string{"type": "string", "description": "Resource namespace"},
					"name":      map[string]string{"type": "string", "description": "Resource name"},
				},
				"required": []string{"kind", "namespace", "name"},
			},
		},
	}
}

func stringParam(params map[string]any, key string) (string, error) {
	v, ok := params[key]
	if !ok {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string, got %T", key, v)
	}
	return s, nil
}

func executeTool(ctx context.Context, exec *kube.Executor, name string, input json.RawMessage) (string, error) {
	var params map[string]any
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("unmarshal tool input: %w", err)
	}

	switch name {
	case "describe_pod":
		ns, err := stringParam(params, "namespace")
		if err != nil {
			return "", err
		}
		n, err := stringParam(params, "name")
		if err != nil {
			return "", err
		}
		return exec.DescribePod(ctx, ns, n)

	case "get_pod_logs":
		ns, err := stringParam(params, "namespace")
		if err != nil {
			return "", err
		}
		n, err := stringParam(params, "name")
		if err != nil {
			return "", err
		}
		tailLines := 100
		if v, ok := params["tail_lines"].(float64); ok {
			tailLines = int(v)
		}
		return exec.GetPodLogs(ctx, ns, n, tailLines)

	case "get_events":
		ns, err := stringParam(params, "namespace")
		if err != nil {
			return "", err
		}
		n, err := stringParam(params, "name")
		if err != nil {
			return "", err
		}
		return exec.GetEvents(ctx, ns, n)

	case "get_node_status":
		nodeName, err := stringParam(params, "node_name")
		if err != nil {
			return "", err
		}
		return exec.GetNodeStatus(ctx, nodeName)

	case "get_resource":
		kind, err := stringParam(params, "kind")
		if err != nil {
			return "", err
		}
		ns, err := stringParam(params, "namespace")
		if err != nil {
			return "", err
		}
		n, err := stringParam(params, "name")
		if err != nil {
			return "", err
		}
		return exec.GetResource(ctx, kind, ns, n)

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
