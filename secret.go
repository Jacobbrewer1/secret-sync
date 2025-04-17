package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kubeErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrNoMount                = errors.New("mount is required")
	ErrNoName                 = errors.New("name is required")
	ErrNoDestinationNamespace = errors.New("destination_namespace is required")
	ErrNoDestinationName      = errors.New("destination_name is required")
)

type Secret struct {
	Mount                string            `mapstructure:"mount"`
	Name                 string            `mapstructure:"name"`
	DestinationNamespace string            `mapstructure:"destination_namespace"`
	DestinationName      string            `mapstructure:"destination_name"`
	Type                 corev1.SecretType `mapstructure:"type"` // Should be a Kubernetes Secret type
}

func (s *Secret) Valid() error {
	switch {
	case s.Mount == "":
		return ErrNoMount
	case s.Name == "":
		return ErrNoName
	case s.DestinationNamespace == "":
		return ErrNoDestinationNamespace
	case s.DestinationName == "":
		return ErrNoDestinationName
	default:
		return nil
	}
}

func (s *Secret) Upsert(ctx context.Context, kubeClient kubernetes.Interface, value map[string]any) error {
	// Create a new Kubernetes Secret
	newSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.DestinationName,
			Namespace:   s.DestinationNamespace,
			Annotations: make(map[string]string),
			Labels: map[string]string{
				secretLabelManagedBy: appName,
			},
		},
		Type: corev1.SecretTypeOpaque, // Default to opaque
	}

	if s.Type != "" {
		newSecret.Type = s.Type
	}

	newSecret.Data = make(map[string][]byte)
	for vk, vv := range value {
		newSecret.Data[vk] = []byte(fmt.Sprintf("%v", vv))
	}
	if len(newSecret.Data) == 0 {
		return errors.New("no data found in secret")
	}

	// Add an annotation with the hash of the Secret
	hashBytes, err := json.Marshal(newSecret)
	if err != nil {
		return fmt.Errorf("error marshalling secret data: %w", err)
	}
	hash := shaHash(hashBytes)
	newSecret.Annotations[secretAnnotationSyncIdKey] = hash

	// Does the secret already exist?
	existingSecret, err := kubeClient.CoreV1().Secrets(s.DestinationNamespace).Get(ctx, s.DestinationName, metav1.GetOptions{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
	})
	if err != nil && kubeErr.IsNotFound(err) {
		// Try to create the Secret first
		_, err = kubeClient.CoreV1().Secrets(s.DestinationNamespace).Create(ctx, newSecret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating secret: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("error getting existing secret: %w", err)
	}

	if existingSecret.Labels == nil {
		existingSecret.Labels = make(map[string]string)
	} else if existingSecret.Labels[secretLabelManagedBy] != appName {
		return fmt.Errorf("secret %s/%s is not managed by %s", s.DestinationNamespace, s.DestinationName, appName)
	}

	if existingSecret.Annotations == nil {
		existingSecret.Annotations = make(map[string]string)
	} else if existingSecret.Annotations[secretAnnotationSyncIdKey] == hash {
		// The secret already exists and is up to date
		return nil
	}

	existingSecret.Labels = newSecret.Labels
	existingSecret.Annotations = newSecret.Annotations
	existingSecret.Type = newSecret.Type
	existingSecret.Data = newSecret.Data

	_, err = kubeClient.CoreV1().Secrets(s.DestinationNamespace).Update(ctx, existingSecret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating secret: %w", err)
	}

	return nil
}
