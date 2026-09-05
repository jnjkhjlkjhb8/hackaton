package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type BatchWriter interface {
	StoreBatch(context.Context, string, Batch) error
}

type WriterFunc func(context.Context, string, Batch) error

func (f WriterFunc) StoreBatch(ctx context.Context, masterID string, batch Batch) error {
	return f(ctx, masterID, batch)
}

type Consumer struct {
	writer BatchWriter
	logger *slog.Logger
}

func NewConsumer(writer BatchWriter, logger *slog.Logger) *Consumer {
	return &Consumer{writer: writer, logger: logger}
}

func (c *Consumer) Handle(_ mqtt.Client, message mqtt.Message) {
	masterID, err := masterIDFromTopic(message.Topic())
	if err != nil {
		c.logger.Warn("reject telemetry topic", "topic", message.Topic(), "err", err)
		return
	}
	var batch Batch
	if err := json.Unmarshal(message.Payload(), &batch); err != nil {
		c.logger.Warn("reject telemetry payload", "master_id", masterID, "err", err)
		return
	}
	if err := batch.Validate(); err != nil {
		c.logger.Warn("reject telemetry batch", "master_id", masterID, "err", err)
		return
	}
	if err := c.writer.StoreBatch(context.Background(), masterID, batch); err != nil {
		c.logger.Error("store telemetry batch", "master_id", masterID, "err", err)
	}
}

func masterIDFromTopic(topic string) (string, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 5 || parts[0] != "farm" || parts[1] != "v1" || parts[2] != "masters" || parts[4] != "telemetry" || parts[3] == "" {
		return "", fmt.Errorf("unexpected topic %q", topic)
	}
	return parts[3], nil
}
