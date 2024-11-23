//go:build wireinject
// +build wireinject

package main

import "github.com/google/wire"

func InitializeApp() (App, error) {
	wire.Build(
		getRootContext,
		getKubeClient,
		getConfig,
		getVaultClient,
		getWorkerPool,
		newApp,
	)
	return new(app), nil
}
