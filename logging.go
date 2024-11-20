package main

import (
	"log/slog"
	"os"
	"strings"
)

func initializeLogger() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: replaceLogAttrs,
	}))

	logger.With(
		loggingKeyAppName, appName,
	)

	slog.SetDefault(logger)
}

// replaceLogAttrs is a slog.HandlerOptions.ReplaceAttr function that replaces some attributes.
func replaceLogAttrs(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.SourceKey:
		// Cut the source file to a relative path.
		v := strings.Split(a.Value.String(), "/")
		idx := len(v) - 2
		if idx < 0 {
			idx = 0
		}
		a.Value = slog.StringValue(strings.Join(v[idx:], "/"))

		// Remove any curly braces from the source file. This is needed for the logstash parser.
		a.Value = slog.StringValue(strings.ReplaceAll(a.Value.String(), "{", ""))
		a.Value = slog.StringValue(strings.ReplaceAll(a.Value.String(), "}", ""))
	}
	return a
}
