# `pkg/kube` — Kubernetes client helpers for rollout control

`pkg/kube` is a lightweight Go package that provides Kubernetes client utilities for managing and inspecting Kubernetes workloads (Deployments, StatefulSets, DaemonSets, and Pods). Its primary purpose is to enable programmatic **rollout restarts** of workloads and structured inspection of Pod details.

This package simplifies common controller actions — like triggering rollouts or checking rollout status — without manually interacting with client-go APIs.

---

## 🚀 Features

* **Rollout restart support** — restart Deployments, StatefulSets, and DaemonSets
* **Flexible targeting** — select targets by:

  * owning controller
  * pod name
  * label selector
  * explicit workload reference
* **Optional wait for rollout completion**
* **Poll interval configuration for status checks**
* **Pod info helpers** for easy inspection
* **DRY, type-safe APIs using Go’s Kubernetes client interfaces**

---

## 📦 Installation

From your module root:

```bash
go get github.com/kwtucker/reef/pkg/kube
```

Ensure your module also depends on Kubernetes client modules such as:

```bash
go get k8s.io/client-go@latest
"go get k8s.io/apimachinery@latest
```

---

## 🎯 Concepts

### WorkloadKind

Represents a Kubernetes workload type that can be restarted:

```go
type WorkloadKind string

const (
	Deployment  WorkloadKind = "Deployment"
	StatefulSet WorkloadKind = "StatefulSet"
	DaemonSet   WorkloadKind = "DaemonSet"
)
```

---

## 📌 Main API Overview

### Creating a Client

Wraps a Kubernetes clientset with defaults:

```go
client, err := kube.NewClient(context.Background(), "my-namespace")
```

If `namespace` is empty, it will detect the in-cluster namespace or fall back to default.

---

### Restarting a Workload

Trigger a controlled rollout restart:

```go
err := client.RolloutRestart(kube.RestartOptions{
	Target: kube.RestartTarget{
		LabelSelector: "app=myapp",
	},
	Wait:    true,                     // wait for rollout
	Timeout: 3 * time.Minute,
})
```

Valid targets:

* `Self` — restart the workload owning the current pod
* `PodName` — restart the controller owning a specific pod
* `LabelSelector` — restart by label selector
* `Controller` — restart a named controller directly

---

### Waiting for Rollout

Polls the specified resource until the rollout stabilizes:

```go
err := client.WaitForRollout(
	context.Background(),
	kube.Deployment,
	"myapp-deployment",
	5*time.Minute,
	2*time.Second,
)
```

The waiter adapts its logic for each WorkloadKind:

* Deployments wait for updated & available replicas
* StatefulSets wait for updated & ready replicas
* DaemonSets ensure desired number of nodes are updated

---

## 🧩 Pod Helpers

### Fetching a Pod

```go
pod, _ := client.Pod("my-pod-name")
```

If the name is empty, it falls back to the local environment’s hostname (useful in-cluster).

---

### Inspecting Pod Info

Convert a Pod into structured metadata:

```go
info := pod.Info()
fmt.Println(info.Node, info.Ready, info.Containers)
```

This includes:

* Spec containers
* Labels
* Node, Namespace, Hostname
* Ready status

---

## 🧪 Example Usage

```go
client, _ := kube.NewClient(context.Background(), "default")

// Restart using a label selector
err := client.RolloutRestart(kube.RestartOptions{
	Target: kube.RestartTarget{
		LabelSelector: "app=api",
	},
	Wait: true,
})
```

---

## ⚙️ Under the Hood

* Uses Kubernetes client-go interfaces for type-safe API calls
* Resolves ReplicaSets to their owning Deployments during restart
* Retries on conflicts using `RetryOnConflict`
* Polls rollout status instead of deleting individual pods

---

## 📄 Contributing

Contributions, issues, and feature requests are welcome!
Feel free to open an issue or submit a pull request on the main repo.

---

## 📜 License

MIT License — see [LICENSE](../../LICENSE) for details.
