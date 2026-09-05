// Package config loads Host process configuration from its environment.
package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL   string
	HTTPAddr      string
	MQTTBrokerURL string
	MQTTUsername  string
	MQTTPassword  string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		HTTPAddr:      os.Getenv("HTTP_ADDR"),
		MQTTBrokerURL: os.Getenv("MQTT_BROKER_URL"),
		MQTTUsername:  os.Getenv("MQTT_USERNAME"),
		MQTTPassword:  os.Getenv("MQTT_PASSWORD"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if cfg.MQTTBrokerURL == "" {
		return Config{}, errors.New("MQTT_BROKER_URL is required")
	}
	return cfg, nil
}
