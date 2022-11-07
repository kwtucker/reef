package kube

import (
	"context"
	"io/ioutil"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	namespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

type Client struct {
	ctx       context.Context
	namespace string
	clientSet *kubernetes.Clientset
}

// NewClient will create a new kubernetes client.
func NewClient(ctx context.Context, namespace string) (Client, error) {
	clientset, err := GetKubeClientset()
	if err != nil {
		return Client{}, err
	}

	if namespace == "" {
		namespace, err = GetNamespace()
		if err != nil {
			return Client{}, err
		}
	}

	return Client{
		ctx:       ctx,
		namespace: namespace,
		clientSet: clientset,
	}, nil

}

// GetKubeClientset will create a new kubernetes clientset.
func GetKubeClientset() (*kubernetes.Clientset, error) {
	var kubeconfig string

	if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
		kubeconfig = clientcmd.RecommendedHomeFile
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)

	return clientSet, err
}

// GetNamespace returns a namespace name.
func GetNamespace() (string, error) {
	ns, err := ioutil.ReadFile(namespacePath)
	if err != nil {
		return "", err
	}

	return string(ns), nil
}
