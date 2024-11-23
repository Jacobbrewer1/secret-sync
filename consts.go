package main

const (
	appName = "secret-sync"

	loggingKeyAppName = "app"
	loggingKeyError   = "err"

	defaultKubeConfigLocation     = "$HOME/.kube/config"
	defaultRefreshIntervalSeconds = 30
	defaultVaultAddr              = "http://vault-active.vault.svc.cluster.local:8200"

	secretAnnotationKey  = "vault-sync-id"
	secretAnnotationPath = "path"
)
