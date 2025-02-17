package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	vault "github.com/hashicorp/vault/api"
	"github.com/jacobbrewer1/workerpool"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type taskUpdateSecret struct {
	ctx context.Context
	kc  *kubernetes.Clientset
	vc  *vault.Client
	s   *secret
}

func newTaskUpdateSecret(
	ctx context.Context,
	kc *kubernetes.Clientset,
	vc *vault.Client,
	secret *secret,
) workerpool.Runnable {
	return &taskUpdateSecret{
		ctx: ctx,
		kc:  kc,
		vc:  vc,
		s:   secret,
	}
}

func (t *taskUpdateSecret) Run() {
	// Get the existing secret
	existingSecret, err := t.kc.CoreV1().Secrets(t.s.DestinationNamespace).Get(t.ctx, t.s.DestinationName, metav1.GetOptions{})
	if err != nil {
		slog.Error("Error getting existing secret", slog.String(loggingKeyError, err.Error()))
		return
	}

	existingSecret.Labels = map[string]string{
		secretLabelManagedBy: appName,
	}

	// Get the secret from Vault
	vaultSecret, err := t.vc.Logical().Read(t.s.Path)
	if err != nil {
		slog.Error("Error reading secret from Vault", slog.String(loggingKeyError, err.Error()))
		return
	}

	// Add the secret data to the Kubernetes secret
	existingSecret.Data = make(map[string][]byte)
	for vk, vv := range vaultSecret.Data {
		if vk != "data" {
			continue
		}

		m, ok := vv.(map[string]any)
		if !ok {
			slog.Error("Error casting secret data to map[string]interface{}")
			return
		}

		for k, v := range m {
			existingSecret.Data[k] = []byte(fmt.Sprintf("%v", v))
		}
	}

	hashBytes, err := json.Marshal(existingSecret.Data)
	if err != nil {
		slog.Error("Error marshalling secret data", slog.String(loggingKeyError, err.Error()))
		return
	}
	hash := shaHash(hashBytes)

	if existingSecret.Annotations == nil {
		existingSecret.Annotations = map[string]string{
			secretAnnotationKey: hash,
		}
	} else if existingSecret.Annotations[secretAnnotationKey] == hash &&
		(existingSecret.Labels != nil && existingSecret.Labels[secretLabelManagedBy] == appName) {
		slog.Debug("Secret is up to date",
			slog.String(loggingKeyNamespace, t.s.DestinationNamespace),
			slog.String(loggingKeyDestination, t.s.DestinationName),
		)
		return
	}

	existingSecret.Annotations[secretAnnotationKey] = hash

	_, err = t.kc.CoreV1().Secrets(t.s.DestinationNamespace).Update(t.ctx, existingSecret, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("Error updating secret", slog.String(loggingKeyError, err.Error()))
		return
	}

	slog.Debug("Secret updated successfully",
		slog.String(loggingKeyNamespace, t.s.DestinationNamespace),
		slog.String(loggingKeyDestination, t.s.DestinationName),
	)
}
