package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	coreErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ErrNoPath                 = errors.New("path is required")
	ErrNoDestinationNamespace = errors.New("destination_namespace is required")
	ErrNoDestinationName      = errors.New("destination_name is required")
)

type secret struct {
	Path                 string `mapstructure:"path"`
	DestinationNamespace string `mapstructure:"destination_namespace"`
	DestinationName      string `mapstructure:"destination_name"`
}

func (s *secret) Valid() error {
	if s.Path == "" {
		return ErrNoPath
	} else if s.DestinationNamespace == "" {
		return ErrNoDestinationNamespace
	} else if s.DestinationName == "" {
		return ErrNoDestinationName
	}

	return nil
}

func (a *app) watchSecrets() {
	refreshInterval := a.config.GetInt("refresh_interval")
	if refreshInterval == 0 {
		refreshInterval = defaultRefreshIntervalSeconds
	}

	ticker := time.NewTicker(time.Duration(refreshInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.syncSecrets()
		}
	}
}

// All secrets will have the annotation of "vault-sync-id=hash" where hash is the hash of the path.
func (a *app) syncSecrets() {
	// Unmarshal the secrets into a slice of secret structs
	secrets := make([]secret, 0)
	if err := a.config.UnmarshalKey("secrets", &secrets); err != nil {
		slog.Error("Error unmarshalling secrets", slog.String(loggingKeyError, err.Error()))
		return
	}

	for _, s := range secrets {
		if err := s.Valid(); err != nil {
			slog.Error("Invalid secret", slog.String(loggingKeyError, err.Error()))
			return
		}

		// Get all namespaces
		namespaces, err := a.client.CoreV1().Namespaces().List(a.ctx, metav1.ListOptions{})
		if err != nil {
			slog.Error("Error getting namespaces", slog.String(loggingKeyError, err.Error()))
			return
		}

		secretFound := false
		for _, ns := range namespaces.Items {
			// Does the secret exist in this namespace?
			foundSecret, err := a.client.CoreV1().Secrets(ns.Name).Get(a.ctx, s.DestinationName, metav1.GetOptions{
				TypeMeta: metav1.TypeMeta{
					Kind: "Secret",
				},
			})
			if err != nil {
				newErr := new(coreErr.StatusError)
				if errors.As(err, &newErr) && newErr.ErrStatus.Reason == metav1.StatusReasonNotFound {
					// Secret does not exist in this namespace
					continue
				}

				slog.Error("Error getting secret", slog.String(loggingKeyError, err.Error()))
				return
			} else if foundSecret.Namespace != s.DestinationNamespace {
				slog.Info("Secret exists in a different namespace", slog.String("namespace", foundSecret.Namespace))

				// Delete the secret
				if err := a.client.CoreV1().Secrets(ns.Name).Delete(a.ctx, foundSecret.Namespace, metav1.DeleteOptions{}); err != nil {
					slog.Error("Error deleting secret", slog.String(loggingKeyError, err.Error()))
					return
				}
			}

			secretFound = true
		}

		if !secretFound {
			a.createKubeSecret(&s)
		} else {
			a.updateKubeSecret(&s)
		}
	}
}

func (a *app) createKubeSecret(s *secret) {
	// Create a new Kubernetes secret
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.DestinationName,
			Namespace:   s.DestinationNamespace,
			Annotations: map[string]string{},
		},
	}

	// Get the secret from Vault
	vaultSecret, err := a.vc.Logical().Read(s.Path)
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
	hashBytes, err := json.Marshal(newSecret.Data)
	if err != nil {
		slog.Error("Error marshalling secret data", slog.String(loggingKeyError, err.Error()))
		return
	}
	hash := shaHash(hashBytes)
	newSecret.Annotations[secretAnnotationKey] = hash

	_, err = a.client.CoreV1().Secrets(s.DestinationNamespace).Create(a.ctx, newSecret, metav1.CreateOptions{})
	if err != nil {
		slog.Error("Error creating secret", slog.String(loggingKeyError, err.Error()))
		return
	}

	slog.Info("Secret created successfully", slog.String("namespace", s.DestinationNamespace), slog.String("name", s.DestinationName))
}

func (a *app) updateKubeSecret(s *secret) {
	// Get the existing secret
	existingSecret, err := a.client.CoreV1().Secrets(s.DestinationNamespace).Get(a.ctx, s.DestinationName, metav1.GetOptions{})
	if err != nil {
		slog.Error("Error getting existing secret", slog.String(loggingKeyError, err.Error()))
		return
	}

	// Get the secret from Vault
	vaultSecret, err := a.vc.Logical().Read(s.Path)
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

		m, ok := vv.(map[string]interface{})
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
	} else if existingSecret.Annotations[secretAnnotationKey] == hash {
		slog.Info("Secret is up to date", slog.String("namespace", s.DestinationNamespace), slog.String("name", s.DestinationName))
		return
	}

	existingSecret.Annotations[secretAnnotationKey] = hash

	_, err = a.client.CoreV1().Secrets(s.DestinationNamespace).Update(a.ctx, existingSecret, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("Error updating secret", slog.String(loggingKeyError, err.Error()))
		return
	}

	slog.Info("Secret updated successfully", slog.String("namespace", s.DestinationNamespace), slog.String("name", s.DestinationName))
}
