package kube

import (
	"fmt"
	apiCorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"os"
)

func (c Client) RestartPod(hostname string) error {

	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		p, err := c.Pod(hostname)
		if err != nil {
			return err
		}

		return c.clientSet.CoreV1().Pods(c.namespace).Delete(c.ctx, p.pod.GetName(), metav1.DeleteOptions{})
	})

	if err != nil {
		return fmt.Errorf("unable to delete %s with error; %w", hostname, err)
	}

	return nil
}

func (c Client) Pod(hostname string) (Pod, error) {
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	p, err := c.clientSet.CoreV1().Pods(c.namespace).Get(c.ctx, hostname, metav1.GetOptions{})
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

	for _, c := range p.pod.Spec.Containers {
		pi.Containers = append(pi.Containers, Container{
			Name:  c.Name,
			Image: c.Image,
		})
	}

	pi.Labels = p.pod.Labels
	pi.Node = p.pod.Spec.NodeName
	pi.Namespace = p.pod.GetNamespace()
	pi.Hostname = p.pod.GetName()
	pi.Ready = p.Ready()
	if pi.Hostname == "" {
		pi.Hostname, _ = os.Hostname()
	}

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
