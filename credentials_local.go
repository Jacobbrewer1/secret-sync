//go:build local
// +build local

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func getKubeClient() (*kubernetes.Clientset, error) {
	configLoc := os.Getenv(envKeyKubeConfigLocation)
	if configLoc == "" {
		configLoc = defaultKubeConfigLocation
	}

	kubeconfigloc := filepath.Clean(configLoc)

	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigloc)
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig from flags: %w", err)
	}

	client := new(kubernetes.Clientset)
	if client, err = kubernetes.NewForConfig(kubeconfig); err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %w", err)
	}

	return client, nil
}
