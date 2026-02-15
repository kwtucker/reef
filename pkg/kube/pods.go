package kube

import (
	"os"

	apiCorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (c *Client) RestartPod(podName string) error {
	if podName == "" {
		podName, _ = os.Hostname()
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return c.kube.
			CoreV1().
			Pods(c.ns).
			Delete(c.ctx, podName, metav1.DeleteOptions{})
	})
}

func (c *Client) Pod(podName string) (Pod, error) {
	if podName == "" {
		podName, _ = os.Hostname()
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

type Pod struct {
	pod *apiCorev1.Pod
}

type Container struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type PodInfo struct {
	Node       string            `json:"node"`
	Namespace  string            `json:"namespace"`
	Hostname   string            `json:"hostname"`
	Ready      bool              `json:"ready"`
	Containers []Container       `json:"containers"`
	Labels     map[string]string `json:"labels"`
}

func (p Pod) Pod() *apiCorev1.Pod {
	return p.pod
}

func (p Pod) Info() PodInfo {
	pi := PodInfo{}

	for _, ctr := range p.pod.Spec.Containers {
		pi.Containers = append(pi.Containers, Container{
			Name:  ctr.Name,
			Image: ctr.Image,
		})
	}

	pi.Labels = p.pod.Labels
	pi.Node = p.pod.Spec.NodeName
	pi.Namespace = p.pod.GetNamespace()
	pi.Hostname = p.pod.GetName()
	pi.Ready = p.Ready()

	return pi
}

func (p Pod) Ready() bool {
	for _, condition := range p.pod.Status.Conditions {
		switch condition.Type {
		case apiCorev1.PodReady:
			if condition.Status == apiCorev1.ConditionTrue {
				return true
			}
		}
	}
	return false
}
