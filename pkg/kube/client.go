package kube

import (
	"context"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// WorkloadKind represents the type of a Kubernetes controller/workload
// that can be restarted. Used for unified handling of deployments,
// statefulsets, and daemonsets.
type WorkloadKind string

const (
	Deployment  WorkloadKind = "Deployment"
	StatefulSet WorkloadKind = "StatefulSet"
	DaemonSet   WorkloadKind = "DaemonSet"
)

// Client wraps a Kubernetes clientset with a namespace and context
// for performing rollout restarts and queries.
type Client struct {
	ctx  context.Context
	ns   string
	kube kubernetes.Interface
}

// NewClient constructs a new kube Client. If namespace is empty, it
// will attempt to discover it automatically.
func NewClient(ctx context.Context, namespace string) (*Client, error) {
	kube, err := GetKubeClientset()
	if err != nil {
		return nil, err
	}

	if namespace == "" {
		namespace = GetNamespace()
	}

	return &Client{
		ctx:  ctx,
		ns:   namespace,
		kube: kube,
	}, nil
}

// RestartTarget defines the target for a rollout restart.
// Only one field should be set at a time.
type RestartTarget struct {
	// Self restarts the workload that owns the current pod.
	Self bool

	// PodName restarts the controller that owns a specific pod.
	PodName string

	// LabelSelector restarts the workload owning pods matching this label selector.
	LabelSelector string

	// Controller explicitly restarts the specified workload.
	Controller *ControllerRef
}

// ControllerRef is a reference to a known Kubernetes controller.
type ControllerRef struct {
	Kind WorkloadKind // Deployment, StatefulSet, DaemonSet
	Name string
}

// RestartOptions defines options for a rollout restart.
type RestartOptions struct {
	Target       RestartTarget // The target workload
	DryRun       bool          // If true, simulate restart without changing anything
	PollInterval time.Duration // How often to poll for rollout status
	Wait         bool          // If true, wait for rollout to complete
	Timeout      time.Duration // Maximum time to wait for rollout
}

// RolloutRestart performs a restart of the specified target workload
// according to the RestartOptions provided.
func (c *Client) RolloutRestart(opts RestartOptions) error {
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}

	// Validate that exactly one target is specified
	if err := validateRestartTarget(opts.Target); err != nil {
		return err
	}

	switch {
	case opts.Target.Controller != nil:
		return c.restartControllerRef(opts)
	case opts.Target.LabelSelector != "":
		return c.restartBySelector(opts)
	case opts.Target.Self:
		return c.restartSelf(opts)
	case opts.Target.PodName != "":
		return c.restartFromPodName(opts)
	default:
		return fmt.Errorf("no restart target specified")
	}
}

// WaitForRollout waits until the rollout of the specified workload
// kind and name is complete, polling at pollInterval for up to timeout.
func (c *Client) WaitForRollout(
	ctx context.Context,
	kind WorkloadKind,
	name string,
	timeout time.Duration,
	pollInterval time.Duration,
) error {
	if pollInterval == 0 {
		pollInterval = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %s/%s rollout", kind, name)
		case <-ticker.C:
			ready, err := c.isRolloutReady(ctx, kind, name)
			if err != nil {
				return err
			}
			if ready {
				return nil
			}
		}
	}
}

// isRolloutReady checks whether the rollout for the specified workload
// kind and name has completed successfully.
func (c *Client) isRolloutReady(ctx context.Context, kind WorkloadKind, name string) (bool, error) {
	switch kind {
	case Deployment:
		d, err := c.kube.AppsV1().Deployments(c.ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return d.Status.UpdatedReplicas == *d.Spec.Replicas &&
			d.Status.AvailableReplicas == *d.Spec.Replicas, nil
	case StatefulSet:
		s, err := c.kube.AppsV1().StatefulSets(c.ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return s.Status.UpdatedReplicas == *s.Spec.Replicas &&
			s.Status.ReadyReplicas == *s.Spec.Replicas, nil
	case DaemonSet:
		d, err := c.kube.AppsV1().DaemonSets(c.ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return d.Status.UpdatedNumberScheduled == d.Status.DesiredNumberScheduled &&
			d.Status.NumberReady == d.Status.DesiredNumberScheduled, nil
	}
	return false, fmt.Errorf("unsupported kind %q", kind)
}

// validateRestartTarget ensures that exactly one target is set in RestartTarget.
func validateRestartTarget(t RestartTarget) error {
	count := 0
	if t.Self {
		count++
	}
	if t.PodName != "" {
		count++
	}
	if t.LabelSelector != "" {
		count++
	}
	if t.Controller != nil {
		count++
	}

	if count == 0 {
		return fmt.Errorf("exactly one restart target must be specified")
	}
	if count > 1 {
		return fmt.Errorf("restart targets are mutually exclusive")
	}
	return nil
}

// restartSelf restarts the workload that owns the current pod.
func (c *Client) restartSelf(opts RestartOptions) error {
	podName, err := os.Hostname()
	if err != nil {
		return err
	}
	return c.restartFromPodName(RestartOptions{
		Target: RestartTarget{
			PodName: podName,
		},
		DryRun:  opts.DryRun,
		Wait:    opts.Wait,
		Timeout: opts.Timeout,
	})
}

// restartFromPodName restarts the controller that owns the specified pod.
func (c *Client) restartFromPodName(opts RestartOptions) error {
	pod, err := c.kube.CoreV1().Pods(c.ns).Get(c.ctx, opts.Target.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	owner := metav1.GetControllerOf(pod)
	if owner == nil {
		return fmt.Errorf("pod %s has no owning controller", pod.Name)
	}

	return c.restartController(owner, opts)
}

// restartBySelector restarts the controller owning pods matching the label selector.
func (c *Client) restartBySelector(opts RestartOptions) error {
	pods, err := c.kube.CoreV1().Pods(c.ns).List(c.ctx, metav1.ListOptions{
		LabelSelector: opts.Target.LabelSelector,
	})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for selector %q", opts.Target.LabelSelector)
	}

	owner := metav1.GetControllerOf(&pods.Items[0])
	if owner == nil {
		return fmt.Errorf("pods have no owning controller")
	}

	return c.restartController(owner, opts)
}

// restartControllerRef restarts the controller explicitly specified in ControllerRef.
func (c *Client) restartControllerRef(opts RestartOptions) error {
	ref := opts.Target.Controller

	return c.restartController(&metav1.OwnerReference{
		Kind: string(ref.Kind),
		Name: ref.Name,
	}, opts)
}

// restartController restarts the workload referenced by the OwnerReference,
// automatically resolving ReplicaSets to their parent Deployments.
func (c *Client) restartController(owner *metav1.OwnerReference, opts RestartOptions) error {
	restartedAt := time.Now().Format(time.RFC3339)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var kind WorkloadKind
		var name string

		switch owner.Kind {
		case "ReplicaSet":
			rs, err := c.kube.AppsV1().ReplicaSets(c.ns).Get(c.ctx, owner.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			parent := metav1.GetControllerOf(rs)
			if parent == nil || parent.Kind != "Deployment" {
				return fmt.Errorf("replicaset %s has no deployment owner", rs.Name)
			}
			kind = Deployment
			name = parent.Name

		case "Deployment":
			kind = Deployment
			name = owner.Name

		case "StatefulSet":
			kind = StatefulSet
			name = owner.Name

		case "DaemonSet":
			kind = DaemonSet
			name = owner.Name

		default:
			return fmt.Errorf("unsupported controller kind %q", owner.Kind)
		}

		return c.restart(name, restartedAt, opts, kind)
	})
}

// patchRestartAnnotation updates the pod template annotation of a workload
// to trigger a rollout restart.
func (c *Client) patchRestartAnnotation(kind WorkloadKind, name, ts string) error {
	switch kind {
	case Deployment:
		obj, err := c.kube.AppsV1().Deployments(c.ns).Get(c.ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if obj.Spec.Template.Annotations == nil {
			obj.Spec.Template.Annotations = map[string]string{}
		}
		obj.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = ts
		_, err = c.kube.AppsV1().Deployments(c.ns).Update(c.ctx, obj, metav1.UpdateOptions{})
		return err

	case StatefulSet:
		obj, err := c.kube.AppsV1().StatefulSets(c.ns).Get(c.ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if obj.Spec.Template.Annotations == nil {
			obj.Spec.Template.Annotations = map[string]string{}
		}
		obj.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = ts
		_, err = c.kube.AppsV1().StatefulSets(c.ns).Update(c.ctx, obj, metav1.UpdateOptions{})
		return err

	case DaemonSet:
		obj, err := c.kube.AppsV1().DaemonSets(c.ns).Get(c.ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if obj.Spec.Template.Annotations == nil {
			obj.Spec.Template.Annotations = map[string]string{}
		}
		obj.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = ts
		_, err = c.kube.AppsV1().DaemonSets(c.ns).Update(c.ctx, obj, metav1.UpdateOptions{})
		return err
	}
	return fmt.Errorf("unsupported kind %q", kind)
}

// restart performs the actual restart: patches the annotation and optionally waits
// for rollout completion.
func (c *Client) restart(name, ts string, opts RestartOptions, kind WorkloadKind) error {
	if opts.DryRun {
		return nil
	}

	// Patch the pod template annotation to trigger rollout
	if err := c.patchRestartAnnotation(kind, name, ts); err != nil {
		return err
	}

	// Wait for rollout to complete if requested
	if opts.Wait {
		return c.WaitForRollout(c.ctx, kind, name, opts.Timeout, opts.PollInterval)
	}

	return nil
}
