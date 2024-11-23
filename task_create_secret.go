package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	vault "github.com/hashicorp/vault/api"
	"github.com/jacobbrewer1/workerpool"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type taskCreateSecret struct {
	ctx context.Context
	kc  *kubernetes.Clientset
	vc  *vault.Client
	s   *secret
}

func newTaskCreateSecret(
	ctx context.Context,
	kc *kubernetes.Clientset,
	vc *vault.Client,
	secret *secret,
) workerpool.Runnable {
	return &taskCreateSecret{
		ctx: ctx,
		kc:  kc,
		vc:  vc,
		s:   secret,
	}
}

func (t *taskCreateSecret) Run() {
	// Create a new Kubernetes secret
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        t.s.DestinationName,
			Namespace:   t.s.DestinationNamespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				secretLabelManagedBy: appName,
			},
		},
		Type: corev1.SecretTypeOpaque, // Default to opaque
	}

	if t.s.Type != "" {
		newSecret.Type = t.s.Type
	}

	// Get the secret from Vault
	vaultSecret, err := t.vc.Logical().Read(t.s.Path)
	if err != nil {
		slog.Error("Error reading secret from Vault", slog.String(loggingKeyError, err.Error()))
		return
	}

	// Add the secret data to the Kubernetes secret
	newSecret.Data = make(map[string][]byte)
	for vk, vv := range vaultSecret.Data {
		if vk != "data" {
			continue
		}

		m, ok := vv.(map[string]interface{})
		if !ok {
			slog.Error("Error casting secret data to map[string]interface{}")
			return
		}

		for k, v := range m {
			newSecret.Data[k] = []byte(fmt.Sprintf("%v", v))
		}
	}

	// Add an annotation to the secret with the hash of the secret data
	hashBytes, err := json.Marshal(newSecret)
	if err != nil {
		slog.Error("Error marshalling secret data", slog.String(loggingKeyError, err.Error()))
		return
	}
	hash := shaHash(hashBytes)
	newSecret.Annotations[secretAnnotationKey] = hash

	_, err = t.kc.CoreV1().Secrets(t.s.DestinationNamespace).Create(t.ctx, newSecret, metav1.CreateOptions{})
	if err != nil {
		slog.Error("Error creating secret", slog.String(loggingKeyError, err.Error()))
		return
	}

	slog.Debug("Secret created successfully", slog.String("namespace", t.s.DestinationNamespace), slog.String("name", t.s.DestinationName))
}
