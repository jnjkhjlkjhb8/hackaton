// Package config loads Host process configuration from its environment.
package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL                string
	HTTPAddr                   string
	MQTTBrokerURL              string
	MQTTUsername               string
	MQTTPassword               string
	DynSecMQTTBrokerURL        string
	DynSecMQTTUsername         string
	DynSecMQTTPassword         string
	DynSecCAFile               string
	CloudflareAccessTeamDomain string
	CloudflareAccessAudience   string
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	cfg := Config{
		DatabaseURL:                os.Getenv("DATABASE_URL"),
		HTTPAddr:                   os.Getenv("HTTP_ADDR"),
		MQTTBrokerURL:              os.Getenv("MQTT_BROKER_URL"),
		MQTTUsername:               os.Getenv("MQTT_USERNAME"),
		MQTTPassword:               os.Getenv("MQTT_PASSWORD"),
		DynSecMQTTBrokerURL:        os.Getenv("DYNSEC_MQTT_BROKER_URL"),
		DynSecMQTTUsername:         os.Getenv("DYNSEC_MQTT_USERNAME"),
		DynSecMQTTPassword:         os.Getenv("DYNSEC_MQTT_PASSWORD"),
		DynSecCAFile:               os.Getenv("DYNSEC_CA_FILE"),
		CloudflareAccessTeamDomain: os.Getenv("CLOUDFLARE_ACCESS_TEAM_DOMAIN"),
		CloudflareAccessAudience:   os.Getenv("CLOUDFLARE_ACCESS_AUDIENCE"),
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
	if (cfg.DynSecMQTTUsername == "") != (cfg.DynSecMQTTPassword == "") || (cfg.DynSecMQTTUsername != "" && (cfg.DynSecMQTTBrokerURL == "" || cfg.DynSecCAFile == "")) {
		return Config{}, errors.New("Dynamic Security configuration must set all values")
	}
	if partiallyConfigured(cfg.CloudflareAccessTeamDomain, cfg.CloudflareAccessAudience) {
		return Config{}, errors.New("Cloudflare Access configuration must set team domain and audience")
	}
	return cfg, nil
}

func partiallyConfigured(values ...string) bool {
	configured := 0
	for _, value := range values {
		if value != "" {
			configured++
		}
	}
	return configured > 0 && configured != len(values)
}
