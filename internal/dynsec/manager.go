// Package dynsec provisions MQTT identities through Mosquitto Dynamic Security.
package dynsec

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	controlTopic     = "$CONTROL/dynamic-security/v1"
	responseTopic    = "$CONTROL/dynamic-security/v1/response"
	masterRolePrefix = "host-master-telemetry-"
)

type Config struct {
	BrokerURL string
	Username  string
	Password  string
	CAFile    string
}

type Manager struct {
	config Config
}

type command struct {
	Command         string       `json:"command"`
	CorrelationData string       `json:"correlationData"`
	Username        string       `json:"username,omitempty"`
	Password        string       `json:"password,omitempty"`
	ClientID        string       `json:"clientid,omitempty"`
	RoleName        string       `json:"rolename,omitempty"`
	ACLType         string       `json:"acltype,omitempty"`
	Topic           string       `json:"topic,omitempty"`
	Allow           *bool        `json:"allow,omitempty"`
	Priority        int          `json:"priority,omitempty"`
	Roles           []clientRole `json:"roles,omitempty"`
}

type clientRole struct {
	RoleName string `json:"rolename"`
	Priority int    `json:"priority"`
}

type request struct {
	Commands []command `json:"commands"`
}

type response struct {
	Responses []responseCommand `json:"responses"`
}

type responseCommand struct {
	Command         string `json:"command"`
	CorrelationData string `json:"correlationData"`
	Error           string `json:"error"`
}

func New(config Config) (*Manager, error) {
	if config.BrokerURL == "" || config.Username == "" || config.Password == "" || config.CAFile == "" {
		return nil, errors.New("Dynamic Security configuration is incomplete")
	}
	return &Manager{config: config}, nil
}

func (m *Manager) ProvisionMaster(ctx context.Context, masterID, password string) error {
	session, err := m.connect(ctx)
	if err != nil {
		return err
	}
	defer session.client.Disconnect(250)

	roleName := masterRoleName(masterID)
	if err := session.ensureRole(ctx, roleName, masterID); err != nil {
		return err
	}
	if err := session.createOrUpdateClient(ctx, masterID, password, roleName); err != nil {
		return err
	}
	return nil
}

type session struct {
	client    mqtt.Client
	responses <-chan response
}

func (m *Manager) connect(ctx context.Context) (*session, error) {
	tlsConfig, err := tlsConfig(m.config.CAFile)
	if err != nil {
		return nil, err
	}
	responses := make(chan response, 1)
	clientID, err := randomID()
	if err != nil {
		return nil, fmt.Errorf("generate Dynamic Security client ID: %w", err)
	}
	opts := mqtt.NewClientOptions().AddBroker(m.config.BrokerURL).
		SetClientID("host-dynsec-" + clientID).
		SetUsername(m.config.Username).
		SetPassword(m.config.Password).
		SetTLSConfig(tlsConfig).
		SetCleanSession(true).
		SetAutoReconnect(false).
		SetOrderMatters(true).
		SetConnectTimeout(10 * time.Second)
	opts.SetDefaultPublishHandler(func(_ mqtt.Client, message mqtt.Message) {
		if message.Topic() != responseTopic {
			return
		}
		var result response
		if err := json.Unmarshal(message.Payload(), &result); err != nil {
			return
		}
		select {
		case responses <- result:
		default:
		}
	})
	client := mqtt.NewClient(opts)
	if err := waitToken(ctx, client.Connect()); err != nil {
		return nil, fmt.Errorf("connect Dynamic Security client: %w", err)
	}
	if err := waitToken(ctx, client.Subscribe(responseTopic, 1, nil)); err != nil {
		client.Disconnect(250)
		return nil, fmt.Errorf("subscribe Dynamic Security response: %w", err)
	}
	return &session{client: client, responses: responses}, nil
}

func (s *session) ensureRole(ctx context.Context, roleName, masterID string) error {
	if err := s.send(ctx, command{Command: "createRole", RoleName: roleName}); err != nil && !alreadyExists(err) {
		return err
	}
	allow := true
	err := s.send(ctx, command{
		Command:  "addRoleACL",
		RoleName: roleName,
		ACLType:  "publishClientSend",
		Topic:    "farm/v1/masters/" + masterID + "/telemetry",
		Allow:    &allow,
		Priority: 10,
	})
	if err != nil && !alreadyExists(err) {
		return err
	}
	return nil
}

func (s *session) createOrUpdateClient(ctx context.Context, masterID, password, roleName string) error {
	role := []clientRole{{RoleName: roleName, Priority: 10}}
	err := s.send(ctx, command{
		Command:  "createClient",
		Username: masterID,
		Password: password,
		ClientID: masterID,
		Roles:    role,
	})
	if err == nil {
		return nil
	}
	if !alreadyExists(err) {
		return err
	}
	return s.send(ctx, command{
		Command:  "modifyClient",
		Username: masterID,
		Password: password,
		ClientID: masterID,
		Roles:    role,
	})
}

func masterRoleName(masterID string) string {
	return masterRolePrefix + masterID
}

func (s *session) send(ctx context.Context, item command) error {
	correlationData, err := randomID()
	if err != nil {
		return fmt.Errorf("generate Dynamic Security correlation data: %w", err)
	}
	item.CorrelationData = correlationData
	payload, err := json.Marshal(request{Commands: []command{item}})
	if err != nil {
		return fmt.Errorf("encode Dynamic Security command: %w", err)
	}
	if err := waitToken(ctx, s.client.Publish(controlTopic, 1, false, payload)); err != nil {
		return fmt.Errorf("publish Dynamic Security command: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-s.responses:
			for _, responseItem := range result.Responses {
				if responseItem.CorrelationData != correlationData {
					continue
				}
				if responseItem.Error != "" {
					return errors.New(responseItem.Error)
				}
				return nil
			}
		}
	}
}

func tlsConfig(caFile string) (*tls.Config, error) {
	certificate, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read Dynamic Security CA file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certificate) {
		return nil, errors.New("parse Dynamic Security CA file")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}, nil
}

func waitToken(ctx context.Context, token mqtt.Token) error {
	for {
		if token.WaitTimeout(100 * time.Millisecond) {
			return token.Error()
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

func randomID() (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func alreadyExists(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}
