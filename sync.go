package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jacobbrewer1/vaulty"
	"github.com/jacobbrewer1/web"
	"github.com/jacobbrewer1/web/cache"
	corev1 "k8s.io/api/core/v1"
	coreErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeCache "k8s.io/client-go/tools/cache"
)

func watchSecrets(
	l *slog.Logger,
	kubeClient kubernetes.Interface,
	secretInformer kubeCache.SharedIndexInformer,
	vaultClient vaulty.Client,
	hashBucket cache.HashBucket,
	secrets []*Secret,
) web.AsyncTaskFunc {
	return func(ctx context.Context) {
		if _, err := secretInformer.AddEventHandler(kubeCache.ResourceEventHandlerFuncs{
			AddFunc:    nil,
			UpdateFunc: nil,
			DeleteFunc: deletedSecretHandler(ctx, l, kubeClient, vaultClient, hashBucket, secrets),
		}); err != nil {
			l.Error("Error adding event handler", slog.String(loggingKeyError, err.Error()))
			return
		}

		secretInformer.Run(ctx.Done())
	}
}

func deletedSecretHandler(
	ctx context.Context,
	l *slog.Logger,
	kubeClient kubernetes.Interface,
	vaultClient vaulty.Client,
	hashBucket cache.HashBucket,
	secrets []*Secret,
) func(any) {
	return func(obj any) {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return
		}

		if !hashBucket.InBucket(secret.Name) {
			return
		}

		// Check if the secret is a vault secret
		if secret.Labels[secretLabelManagedBy] != appName {
			return
		} else if secret.Annotations[secretAnnotationSyncIdKey] == "" {
			return
		}

		// Recreate the secret as it was deleted
		l.Info("Secret deleted, scheduling recreation",
			slog.String(loggingKeyNamespace, secret.Namespace),
			slog.String(loggingKeyDestination, secret.Name),
		)

		var foundSecret *Secret = nil
		for _, s := range secrets {
			if s.DestinationNamespace != secret.Namespace || s.DestinationName != secret.Name {
				continue
			}
			foundSecret = s
			break
		}
		if foundSecret == nil {
			l.Error("Secret not found in config")
			return
		}

		// Get the secret from vault
		vaultSecret, err := vaultClient.Path(foundSecret.Path).GetKvSecretV2(ctx)
		if err != nil {
			l.Error("Error getting secret from vault", slog.String(loggingKeyError, err.Error()))
			return
		}

		// Upsert the secret
		if err := foundSecret.Upsert(ctx, kubeClient, vaultSecret.Data); err != nil { // nolint:revive // Traditional error handling
			l.Error("Error upserting secret", slog.String(loggingKeyError, err.Error()))
			return
		}
	}
}

// All secrets will have the annotation of "vault-sync-id=hash" where hash is the hash of the path.
func syncSecrets(
	l *slog.Logger,
	kubeClient kubernetes.Interface,
	vaultClient vaulty.Client,
	hashBucket cache.HashBucket,
	interval time.Duration,
	secrets []*Secret,
) web.AsyncTaskFunc {
	return func(ctx context.Context) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				l.Info("Stopping secret sync")
				return
			case <-ticker.C:
				namespaces, err := kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
				if err != nil {
					l.Error("Error listing namespaces", slog.String(loggingKeyError, err.Error()))
					continue
				}

				for _, secret := range secrets {
					if !hashBucket.InBucket(secret.DestinationName) {
						continue
					}

					l = l.With(
						slog.String(loggingKeyNamespace, secret.DestinationNamespace),
						slog.String(loggingKeyDestination, secret.DestinationName),
					)

					if err := secret.Valid(); err != nil {
						l.Error("Invalid secret", slog.String(loggingKeyError, err.Error()))
						continue
					}

					for i := range namespaces.Items {
						ns := &namespaces.Items[i]

						// Does the secret exist in this namespace?
						foundSecret, err := kubeClient.CoreV1().Secrets(ns.Name).Get(ctx, secret.DestinationName, metav1.GetOptions{
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

							l.Error("Error getting secret", slog.String(loggingKeyError, err.Error()))
							continue
						} else if foundSecret.Namespace != secret.DestinationNamespace { // nolint:revive // We need to check if the secret is in the correct namespace
							l.Info("Secret exists in a different namespace", slog.String(loggingKeyNamespace, foundSecret.Namespace))

							// Delete the secret
							if err := kubeClient.CoreV1().Secrets(ns.Name).Delete(ctx, foundSecret.Name, metav1.DeleteOptions{}); err != nil { // nolint:revive // Traditional error handling
								l.Error("Error deleting secret", slog.String(loggingKeyError, err.Error()))
								return
							}
						}
					}

					// Get the secret from vault
					vaultSecret, err := vaultClient.Path(secret.Path).GetKvSecretV2(ctx)
					if err != nil {
						l.Error("Error getting secret from vault", slog.String(loggingKeyError, err.Error()))
						continue
					}

					// Upsert the secret
					if err := secret.Upsert(ctx, kubeClient, vaultSecret.Data); err != nil { // nolint:revive // Traditional error handling
						l.Error("Error upserting secret", slog.String(loggingKeyError, err.Error()))
						continue
					}
				}
			}
		}
	}
}
