package telemetry

import (
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func Start(brokerURL, username, password string, consumer *Consumer) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().AddBroker(brokerURL).SetClientID("host-telemetry-consumer")
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetOnConnectHandler(func(client mqtt.Client) { client.Subscribe("farm/v1/masters/+/telemetry", 1, consumer.Handle) })
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("connect mqtt: %w", token.Error())
	}
	return client, nil
}
