package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
)

// Executor provides read-only Kubernetes operations for diagnostics.
type Executor struct {
	client    kubernetes.Interface
	dynamic   dynamic.Interface
	discovery discovery.DiscoveryInterface
}

func NewExecutor(client kubernetes.Interface, dynClient dynamic.Interface) *Executor {
	return &Executor{client: client, dynamic: dynClient, discovery: client.Discovery()}
}

func (e *Executor) DescribePod(ctx context.Context, namespace, name string) (string, error) {
	pod, err := e.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get pod: %w", err)
	}
	return formatPod(pod), nil
}

func (e *Executor) GetPodLogs(ctx context.Context, namespace, name string, tailLines int) (string, error) {
	tail := int64(tailLines)
	req := e.client.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{
		TailLines: &tail,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("get logs: %w", err)
	}
	defer stream.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		return "", fmt.Errorf("read logs: %w", err)
	}
	return buf.String(), nil
}

func (e *Executor) GetEvents(ctx context.Context, namespace, name string) (string, error) {
	sel := fields.OneTermEqualSelector("regarding.name", name).String()
	list, err := e.client.EventsV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: sel,
	})
	if err != nil {
		return "", fmt.Errorf("list events: %w", err)
	}
	return formatEvents(list.Items), nil
}

func (e *Executor) GetNodeStatus(ctx context.Context, nodeName string) (string, error) {
	node, err := e.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get node: %w", err)
	}
	return formatNode(node), nil
}

// GetResource fetches any Kubernetes resource by kind and returns its status.
// Uses the discovery API to resolve Kind → GVR dynamically, so it works with any CRD.
func (e *Executor) GetResource(ctx context.Context, kind, namespace, name string) (string, error) {
	gvr, err := e.resolveGVR(kind)
	if err != nil {
		return "", fmt.Errorf("resolve kind %s: %w", kind, err)
	}

	obj, err := e.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get %s: %w", kind, err)
	}

	// Extract .status for readability
	status, found := nestedMap(obj.Object, "status")
	if !found {
		status = obj.Object
	}

	out, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal status: %w", err)
	}

	return fmt.Sprintf("Kind: %s\nName: %s\nNamespace: %s\n\nStatus:\n%s", kind, name, namespace, string(out)), nil
}

func (e *Executor) resolveGVR(kind string) (schema.GroupVersionResource, error) {
	groupResources, err := restmapper.GetAPIGroupResources(e.discovery)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("discover API groups: %w", err)
	}

	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	mapping, err := mapper.RESTMapping(schema.GroupKind{Kind: kind})
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("map kind %s: %w", kind, err)
	}

	return mapping.Resource, nil
}

func nestedMap(obj map[string]any, key string) (map[string]any, bool) {
	val, ok := obj[key]
	if !ok {
		return nil, false
	}
	m, ok := val.(map[string]any)
	return m, ok
}

func formatPod(pod *corev1.Pod) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\n", pod.Name)
	fmt.Fprintf(&b, "Namespace: %s\n", pod.Namespace)
	fmt.Fprintf(&b, "Node: %s\n", pod.Spec.NodeName)
	fmt.Fprintf(&b, "Phase: %s\n", pod.Status.Phase)
	fmt.Fprintf(&b, "Reason: %s\n", pod.Status.Reason)
	fmt.Fprintf(&b, "Message: %s\n", pod.Status.Message)

	for _, c := range pod.Status.Conditions {
		fmt.Fprintf(&b, "Condition %s: %s (reason: %s, message: %s)\n",
			c.Type, c.Status, c.Reason, c.Message)
	}
	for _, cs := range pod.Status.ContainerStatuses {
		fmt.Fprintf(&b, "\nContainer: %s\n", cs.Name)
		fmt.Fprintf(&b, "  Image: %s\n", cs.Image)
		fmt.Fprintf(&b, "  Ready: %v\n", cs.Ready)
		fmt.Fprintf(&b, "  RestartCount: %d\n", cs.RestartCount)
		if cs.State.Waiting != nil {
			fmt.Fprintf(&b, "  Waiting: %s (%s)\n", cs.State.Waiting.Reason, cs.State.Waiting.Message)
		}
		if cs.State.Terminated != nil {
			fmt.Fprintf(&b, "  Terminated: %s (exit %d, %s)\n",
				cs.State.Terminated.Reason, cs.State.Terminated.ExitCode, cs.State.Terminated.Message)
		}
		if cs.LastTerminationState.Terminated != nil {
			t := cs.LastTerminationState.Terminated
			fmt.Fprintf(&b, "  LastTermination: %s (exit %d, %s)\n", t.Reason, t.ExitCode, t.Message)
		}
	}
	return b.String()
}

func formatEvents(events []eventsv1.Event) string {
	var b strings.Builder
	for _, ev := range events {
		ts := ev.EventTime.Time
		if ts.IsZero() {
			ts = ev.CreationTimestamp.Time
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\n",
			ts.Format("15:04:05"), ev.Type, ev.Reason, ev.Regarding.Name, ev.Note)
	}
	if b.Len() == 0 {
		return "No events found."
	}
	return b.String()
}

func formatNode(node *corev1.Node) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\n", node.Name)
	for _, c := range node.Status.Conditions {
		fmt.Fprintf(&b, "Condition %s: %s (reason: %s, message: %s)\n",
			c.Type, c.Status, c.Reason, c.Message)
	}
	fmt.Fprintf(&b, "Allocatable CPU: %s\n", node.Status.Allocatable.Cpu())
	fmt.Fprintf(&b, "Allocatable Memory: %s\n", node.Status.Allocatable.Memory())
	return b.String()
}
