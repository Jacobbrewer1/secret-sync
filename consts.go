package main

const (
	appName = "Secret-sync"

	loggingKeyError       = "err"
	loggingKeyNamespace   = "namespace"
	loggingKeyDestination = "destination"

	secretAnnotationSyncIdKey = "vault-sync-id" // nolint:gosec // This is not a credential
	secretLabelManagedBy      = "managed-by"
)
