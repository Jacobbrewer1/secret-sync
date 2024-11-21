package main

import (
	"flag"
	"fmt"

	"github.com/spf13/viper"
)

var (
	configLocation = flag.String("config", "config.json", "The location of the config file")
)

func getConfig() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(*configLocation)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	return v, nil
}
