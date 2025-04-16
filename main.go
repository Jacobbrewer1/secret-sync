package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/fsnotify/fsnotify"
	"github.com/jacobbrewer1/web"
	"github.com/jacobbrewer1/web/logging"
)

type (
	AppConfig struct {
		Secrets      []*Secret
		syncInterval time.Duration
	}

	App struct {
		config *AppConfig
		base   *web.App
	}
)

func NewApp(l *slog.Logger) (*App, error) {
	base, err := web.NewApp(l)
	if err != nil {
		return nil, fmt.Errorf("failed to create web app: %w", err)
	}

	config := new(AppConfig)
	if err := env.Parse(config); err != nil {
		return nil, fmt.Errorf("failed to parse environment: %w", err)
	}

	return &App{
		config: config,
		base:   base,
	}, nil
}

func (a *App) Start() error {
	if err := a.base.Start(
		web.WithVaultClient(),
		web.WithInClusterKubeClient(),
		web.WithKubernetesSecretInformer(),
		web.WithServiceEndpointHashBucket(appName),
		web.WithDependencyBootstrap(func(ctx context.Context) error {
			vip := a.base.Viper()
			vip.OnConfigChange(func(e fsnotify.Event) {
				a.base.Shutdown() // Restart the app on config change
			})
			return nil
		}),
		web.WithDependencyBootstrap(func(ctx context.Context) error {
			vip := a.base.Viper()
			secrets := make([]*Secret, 0)
			if err := vip.UnmarshalKey("secrets", &secrets); err != nil {
				return fmt.Errorf("error unmarshalling secrets: %w", err)
			} else if len(secrets) == 0 {
				return errors.New("no secrets provided")
			}

			for _, secret := range secrets {
				if err := secret.Valid(); err != nil {
					return fmt.Errorf("invalid secret: %w", err)
				}
			}
			a.config.Secrets = secrets
			return nil
		}),
		web.WithDependencyBootstrap(func(ctx context.Context) error {
			interval, err := time.ParseDuration(a.base.Viper().GetString("refresh_interval"))
			if err != nil {
				return fmt.Errorf("failed to parse refresh interval: %w", err)
			} else if interval <= 0 {
				return errors.New("invalid refresh interval")
			}

			a.base.Logger().Info("Interval set", slog.String(loggingKeyInterval, interval.String()))

			a.config.syncInterval = interval
			return nil
		}),
		web.WithIndefiniteAsyncTask("watch-secrets", a.watchSecrets(
			logging.LoggerWithComponent(a.base.Logger(), "watch-secrets"),
		)),
		web.WithIndefiniteAsyncTask("sync-secrets", a.syncSecretsTicker(
			logging.LoggerWithComponent(a.base.Logger(), "sync-secrets"),
		)),
	); err != nil {
		return fmt.Errorf("failed to start web app: %w", err)
	}

	return nil
}

func (a *App) WaitForEnd() {
	a.base.WaitForEnd(a.base.Shutdown)
}

func main() {
	l := logging.NewLogger(
		logging.WithAppName(appName),
	)

	app, err := NewApp(l)
	if err != nil {
		l.Error("failed to create app", slog.Any(logging.KeyError, err))
		panic("failed to create app")
	}

	if err := app.Start(); err != nil {
		l.Error("failed to start app", slog.Any(logging.KeyError, err))
		panic("failed to start app")
	}

	app.WaitForEnd()
}
