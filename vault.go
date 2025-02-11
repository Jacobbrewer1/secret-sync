//go:build !local

package main

import (
	"fmt"
	"log/slog"

	"github.com/hashicorp/vault/api"
	"github.com/jacobbrewer1/vaulty"
	"github.com/spf13/viper"
)

func getVaultClient(v *viper.Viper) (*api.Client, error) {
	addr := v.GetString("vault.address")
	if addr == "" {
		slog.Info("No vault address provided, using default address", slog.String("default_address", defaultVaultAddr))
		addr = defaultVaultAddr
	}

	vc, err := vaulty.NewClient(
		vaulty.WithAddr(addr),
		vaulty.WithKubernetesAuthDefault(),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating vault client: %w", err)
	}

	return vc.Client(), nil
}
