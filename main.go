package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	vault "github.com/hashicorp/vault/api"
	"github.com/jacobbrewer1/workerpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
)

type App interface {
	Start()
}

type app struct {
	ctx    context.Context
	client *kubernetes.Clientset
	config *viper.Viper
	vc     *vault.Client
	wp     workerpool.Pool
}

func newApp(
	ctx context.Context,
	client *kubernetes.Clientset,
	config *viper.Viper,
	vc *vault.Client,
	wp workerpool.Pool,
) App {
	return &app{
		ctx:    ctx,
		client: client,
		config: config,
		vc:     vc,
		wp:     wp,
	}
}

func (a *app) Start() {
	go func() {
		r := mux.NewRouter()
		r.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)
		srv := &http.Server{
			Addr:              ":8080",
			ReadHeaderTimeout: 10 * time.Second,
			Handler:           r,
		}
		slog.Info("Starting metrics server")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) { // nolint:revive // Traditional error handling
			slog.Error("Error starting metrics server", slog.String(loggingKeyError, err.Error()))
			os.Exit(1)
		}
	}()

	go a.watchSecrets()

	refreshInterval := a.config.GetInt("refresh_interval")
	if refreshInterval == 0 {
		refreshInterval = defaultRefreshIntervalSeconds
	}

	ticker := time.NewTicker(time.Duration(refreshInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.syncSecrets()
		}
	}
}

func init() {
	flag.Parse()
	initializeLogger()
}

func main() {
	a, err := InitializeApp()
	if err != nil {
		slog.Error("Error initializing app", slog.String(loggingKeyError, err.Error()))
		os.Exit(1)
	} else if a == nil {
		slog.Error("App is nil")
		os.Exit(1)
	}
	a.Start()
}
