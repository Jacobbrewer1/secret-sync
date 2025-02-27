package main

import (
	"errors"
	"log/slog"

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
	Path                 string            `mapstructure:"path"`
	DestinationNamespace string            `mapstructure:"destination_namespace"`
	DestinationName      string            `mapstructure:"destination_name"`
	Type                 corev1.SecretType `mapstructure:"type"` // Should be a Kubernetes secret type
}

func (s *secret) Valid() error {
	switch {
	case s.Path == "":
		return ErrNoPath
	case s.DestinationNamespace == "":
		return ErrNoDestinationNamespace
	case s.DestinationName == "":
		return ErrNoDestinationName
	default:
		return nil
	}
}

func (a *app) watchSecrets() {
	// Start a kubernetes watcher for secrets
	watcher, err := a.client.CoreV1().Secrets("").Watch(a.ctx, metav1.ListOptions{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				secretLabelManagedBy: appName,
			},
		}),
	})
	if err != nil {
		slog.Error("Error watching secrets", slog.String(loggingKeyError, err.Error()))
		return
	}

	for event := range watcher.ResultChan() {
		a.wp.MustSchedule(newTaskWatcherEvent(event, a))
	}
}

// All secrets will have the annotation of "vault-sync-id=hash" where hash is the hash of the path.
func (a *app) syncSecrets() {
	secrets, err := a.getSecrets()
	if err != nil {
		slog.Error("Error getting secrets", slog.String(loggingKeyError, err.Error()))
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
		for i := range namespaces.Items {
			ns := &namespaces.Items[i] // Make the app less memory intensive

			// Does the secret exist in this namespace?
			foundSecret, err := a.client.CoreV1().Secrets(ns.Name).Get(a.ctx, s.DestinationName, metav1.GetOptions{
				TypeMeta: metav1.TypeMeta{
					Kind: "Secret",
				},
			})
			if err != nil {
				newErr := new(coreErr.StatusError)
				if errors.As(err, &newErr) && newErr.ErrStatus.Reason == metav1.StatusReasonNotFound { // nolint:revive // We need to cast see if the error is a StatusError before we can check the reason
					// Secret does not exist in this namespace
					continue
				}

				slog.Error("Error getting secret", slog.String(loggingKeyError, err.Error()))
				return
			} else if foundSecret.Namespace != s.DestinationNamespace {
				slog.Info("Secret exists in a different namespace", slog.String(loggingKeyNamespace, foundSecret.Namespace))

				// Delete the secret
				if err := a.client.CoreV1().Secrets(ns.Name).Delete(a.ctx, foundSecret.Namespace, metav1.DeleteOptions{}); err != nil {
					slog.Error("Error deleting secret", slog.String(loggingKeyError, err.Error()))
					return
				}
			}

			secretFound = true
		}

		if !secretFound {
			a.wp.MustSchedule(newTaskCreateSecret(a.ctx, a.client, a.vc, s))
		} else {
			a.wp.MustSchedule(newTaskUpdateSecret(a.ctx, a.client, a.vc, s))
		}
	}
}
