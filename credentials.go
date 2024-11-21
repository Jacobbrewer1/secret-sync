//go:build !local

package main

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func getKubeClient() (*kubernetes.Clientset, error) {
	kubeconfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig from flags: %w", err)
	}

	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %w", err)
	}

	return client, nil
}
