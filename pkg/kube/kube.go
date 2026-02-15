package kube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	namespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

type Client struct {
	ctx  context.Context
	ns   string
	kube kubernetes.Interface
}

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

func GetKubeClientset() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := ""
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kube config: %w", err)
		}
	}

	return kubernetes.NewForConfig(config)
}

func GetNamespace() string {
	if ns, err := os.ReadFile(namespacePath); err == nil {
		return strings.TrimSpace(string(ns))
	}

	if home := homedir.HomeDir(); home != "" {
		kubeconfig := filepath.Join(home, ".kube", "config")
		if cfg, err := clientcmd.LoadFromFile(kubeconfig); err == nil {
			if ctx := cfg.Contexts[cfg.CurrentContext]; ctx != nil && ctx.Namespace != "" {
				return ctx.Namespace
			}
		}
	}

	return "default"
}
