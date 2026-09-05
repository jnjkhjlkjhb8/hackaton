// Package enrollment provisions Master and Slave Node identities.
package enrollment

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrMasterExists          = errors.New("master already exists")
	ErrMasterNotFound        = errors.New("master not found")
	ErrMasterNotEnrolled     = errors.New("master is not enrolled")
	ErrMasterDisabled        = errors.New("master is disabled")
	ErrEnrollmentUsed        = errors.New("enrollment token already used")
	ErrInvalidEnrollment     = errors.New("invalid enrollment token")
	ErrInvalidTransfer       = errors.New("invalid transfer token")
	ErrCredentialUnavailable = errors.New("credential provisioning is unavailable")
)

type MasterProvisioning struct {
	MasterID        string `json:"master_id"`
	EnrollmentToken string `json:"enrollment_token"`
}

type MasterCredentials struct {
	MasterID     string `json:"master_id"`
	MQTTUsername string `json:"mqtt_username"`
	MQTTPassword string `json:"mqtt_password"`
}

type SlaveEnrollment struct {
	SlaveID  string `json:"slave_id"`
	MasterID string `json:"master_id"`
	PotID    string `json:"pot_id"`
	Created  bool   `json:"created"`
}

type Repository interface {
	CreateMaster(ctx context.Context, masterID, nodeLabel string, enrollmentTokenHash []byte) error
	EnrollMaster(ctx context.Context, masterID string, enrollmentTokenHash []byte, mqttUsername string, provision func(context.Context) error) error
	EnrollSlave(ctx context.Context, slaveID, masterID, nodeLabel string, transferTokenHash []byte) (SlaveEnrollment, error)
}

type CredentialProvisioner interface {
	ProvisionMaster(ctx context.Context, masterID, password string) error
}

type Service struct {
	repository  Repository
	provisioner CredentialProvisioner
	newToken    func() (string, error)
}

func New(repository Repository, provisioner CredentialProvisioner) *Service {
	return &Service{
		repository:  repository,
		provisioner: provisioner,
		newToken:    newToken,
	}
}

func (s *Service) CreateMasterProvisioning(ctx context.Context, masterID, nodeLabel string) (MasterProvisioning, error) {
	if err := validateID(masterID); err != nil {
		return MasterProvisioning{}, err
	}
	if err := validateLabel(nodeLabel); err != nil {
		return MasterProvisioning{}, err
	}
	token, err := s.newToken()
	if err != nil {
		return MasterProvisioning{}, fmt.Errorf("generate enrollment token: %w", err)
	}
	if err := s.repository.CreateMaster(ctx, masterID, nodeLabel, tokenHash(token)); err != nil {
		return MasterProvisioning{}, err
	}
	return MasterProvisioning{MasterID: masterID, EnrollmentToken: token}, nil
}

func (s *Service) EnrollMaster(ctx context.Context, masterID, enrollmentToken string) (MasterCredentials, error) {
	if err := validateID(masterID); err != nil {
		return MasterCredentials{}, err
	}
	if err := validateSecret(enrollmentToken, "enrollment token"); err != nil {
		return MasterCredentials{}, err
	}
	password, err := s.newToken()
	if err != nil {
		return MasterCredentials{}, fmt.Errorf("generate MQTT password: %w", err)
	}
	if s.provisioner == nil {
		return MasterCredentials{}, ErrCredentialUnavailable
	}
	err = s.repository.EnrollMaster(ctx, masterID, tokenHash(enrollmentToken), masterID, func(ctx context.Context) error {
		return s.provisioner.ProvisionMaster(ctx, masterID, password)
	})
	if err != nil {
		return MasterCredentials{}, err
	}
	return MasterCredentials{MasterID: masterID, MQTTUsername: masterID, MQTTPassword: password}, nil
}

func (s *Service) EnrollSlave(ctx context.Context, slaveID, masterID, nodeLabel, transferToken string) (SlaveEnrollment, error) {
	if err := validateID(slaveID); err != nil {
		return SlaveEnrollment{}, err
	}
	if err := validateID(masterID); err != nil {
		return SlaveEnrollment{}, err
	}
	if err := validateLabel(nodeLabel); err != nil {
		return SlaveEnrollment{}, err
	}
	if err := validateSecret(transferToken, "transfer token"); err != nil {
		return SlaveEnrollment{}, err
	}
	return s.repository.EnrollSlave(ctx, slaveID, masterID, nodeLabel, tokenHash(transferToken))
}

func tokenHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func newToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func validateID(value string) error {
	if len(value) == 0 || len(value) > 64 {
		return errors.New("id must contain 1 to 64 characters")
	}
	for index, runeValue := range value {
		valid := runeValue >= 'a' && runeValue <= 'z' ||
			runeValue >= 'A' && runeValue <= 'Z' ||
			runeValue >= '0' && runeValue <= '9' ||
			runeValue == '-' || runeValue == '_' || runeValue == '.'
		if !valid || (index == 0 && (runeValue == '-' || runeValue == '_' || runeValue == '.')) {
			return errors.New("id may contain only letters, digits, hyphens, underscores, and dots")
		}
	}
	return nil
}

func validateLabel(value string) error {
	if len(value) > 120 {
		return errors.New("node label must contain at most 120 characters")
	}
	return nil
}

func validateSecret(value, name string) error {
	if len(strings.TrimSpace(value)) < 32 || len(value) > 256 {
		return fmt.Errorf("%s must contain 32 to 256 characters", name)
	}
	return nil
}
