package main

import (
	"log/slog"

	"github.com/jacobbrewer1/workerpool"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type taskWatcherEvent struct {
	event watch.Event

	a *app
}

func newTaskWatcherEvent(
	event watch.Event,
	a *app,
) workerpool.Runnable {
	return &taskWatcherEvent{
		event: event,
		a:     a,
	}
}

func (t *taskWatcherEvent) Run() {
	switch t.event.Type {
	case watch.Added, watch.Modified:
		// Do nothing
	case watch.Deleted:
		slog.Debug("Secret deletion event")
		t.handlePodDeleted()
	case watch.Error:
		slog.Error("Watcher event error")
	}
}

// handlePodDeleted recreates the secret when the secret is deleted
func (t *taskWatcherEvent) handlePodDeleted() {
	// Find the secret from the config
	secrets, err := t.a.getSecrets()
	if err != nil {
		slog.Error("Error getting secrets", slog.String(loggingKeyError, err.Error()))
		return
	}

	obj, ok := t.event.Object.(*corev1.Secret)
	if !ok {
		slog.Warn("Error casting object to secret")
		return
	}

	foundSecret := new(secret)
	// Find the secret that matches the deleted secret
	for _, s := range secrets {
		if s.DestinationNamespace == obj.Namespace && s.DestinationName == obj.Name {
			foundSecret = s
			break
		}
	}
	if foundSecret.Path == "" {
		slog.Warn("Secret not found")
		return
	}

	// Create the secret again
	t.a.wp.MustSchedule(newTaskCreateSecret(t.a.ctx, t.a.client, t.a.vc, foundSecret))
}
