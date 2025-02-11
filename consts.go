package main

const (
	appName = "secret-sync"

	loggingKeyAppName = "app"
	loggingKeyError   = "err"
	loggingKeySignal  = "signal"

	defaultKubeConfigLocation     = "$HOME/.kube/config"
	defaultRefreshIntervalSeconds = 30
	defaultVaultAddr              = "http://vault-active.vault.svc.cluster.local:8200"

	secretAnnotationKey  = "vault-sync-id" // nolint:gosec // This is not a credential
	secretLabelManagedBy = "managed-by"
)
