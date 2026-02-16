package kube

import (
	"context"
	"fmt"
	"os"

	apiCorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Pod wraps a Kubernetes Pod object and provides helper methods.
type Pod struct {
	pod  *apiCorev1.Pod
	ctx  context.Context
	ns   string
	kube kubernetes.Interface
}

// Container represents basic container info for JSON/log output.
type Container struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Type  string `json:"type"` // normal, init, ephemeral
}

// PodInfo is a structured view of a Pod, including containers, labels, and status.
type PodInfo struct {
	Node       string            `json:"node"`
	Namespace  string            `json:"namespace"`
	Hostname   string            `json:"hostname"`
	Ready      bool              `json:"ready"`
	Containers []Container       `json:"containers"`
	Labels     map[string]string `json:"labels"`
}

// Pod fetches a pod by name. If podName is empty, defaults to os.Hostname().
func (c *Client) Pod(podName string) (Pod, error) {
	if podName == "" {
		var err error
		podName, err = os.Hostname()
		if err != nil {
			return Pod{}, fmt.Errorf("failed to get hostname: %w", err)
		}
	}

	p, err := c.kube.
		CoreV1().
		Pods(c.ns).
		Get(c.ctx, podName, metav1.GetOptions{})
	if err != nil {
		return Pod{}, err
	}

	return Pod{pod: p}, nil
}

// Pod returns the underlying Kubernetes Pod object.
func (p Pod) Pod() *apiCorev1.Pod {
	return p.pod
}

// Ready returns true if the pod is in Ready condition.
func (p Pod) Ready() bool {
	for _, cond := range p.pod.Status.Conditions {
		if cond.Type == apiCorev1.PodReady && cond.Status == apiCorev1.ConditionTrue {
			return true
		}
	}
	return false
}

// Info builds a complete PodInfo snapshot including init, ephemeral, and regular containers.
func (p Pod) Info() PodInfo {
	pi := PodInfo{
		Node:      p.pod.Spec.NodeName,
		Namespace: p.pod.Namespace,
		Hostname:  p.pod.Name,
		Ready:     p.Ready(),
		Labels:    p.pod.Labels,
	}

	// Regular containers
	for _, c := range p.pod.Spec.Containers {
		pi.Containers = append(pi.Containers, Container{
			Name:  c.Name,
			Image: c.Image,
			Type:  "normal",
		})
	}

	// Init containers
	for _, c := range p.pod.Spec.InitContainers {
		pi.Containers = append(pi.Containers, Container{
			Name:  c.Name,
			Image: c.Image,
			Type:  "init",
		})
	}

	// Ephemeral containers (kubectl debug containers)
	for _, c := range p.pod.Spec.EphemeralContainers {
		pi.Containers = append(pi.Containers, Container{
			Name:  c.Name,
			Image: c.Image,
			Type:  "ephemeral",
		})
	}

	return pi
}
