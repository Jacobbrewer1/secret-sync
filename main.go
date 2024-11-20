package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	vault "github.com/hashicorp/vault/api"
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
}

func newApp(
	ctx context.Context,
	client *kubernetes.Clientset,
	config *viper.Viper,
	vc *vault.Client,
) App {
	return &app{
		ctx:    ctx,
		client: client,
		config: config,
		vc:     vc,
	}
}

func (a *app) Start() {
	go func() {
		r := mux.NewRouter()
		r.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)
		srv := &http.Server{
			Addr:    ":8080",
			Handler: r,
		}
		slog.Info("Starting metrics server")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Error starting metrics server", slog.String(loggingKeyError, err.Error()))
			os.Exit(1)
		}
	}()

	a.watchSecrets()
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
