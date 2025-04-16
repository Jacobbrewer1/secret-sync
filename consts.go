package main

const (
	appName = "secret-sync"

	loggingKeyError       = "err"
	loggingKeyNamespace   = "namespace"
	loggingKeyDestination = "destination"

	secretAnnotationSyncIdKey = "vault-sync-id" // nolint:gosec // This is not a credential
	secretLabelManagedBy      = "managed-by"
)
