package enrollment

import (
	"context"
	"errors"
	"testing"
)

func TestEnrollMasterProvisionsCredentials(t *testing.T) {
	t.Parallel()
	repository := &fakeRepository{}
	provisioner := &fakeProvisioner{}
	service := New(repository, provisioner)
	service.newToken = func() (string, error) { return "12345678901234567890123456789012", nil }

	result, err := service.EnrollMaster(t.Context(), "master-001", "12345678901234567890123456789012")
	if err != nil {
		t.Fatal(err)
	}
	if result.MQTTUsername != "master-001" || result.MQTTPassword != "12345678901234567890123456789012" {
		t.Fatalf("EnrollMaster() = %+v, want generated credentials", result)
	}
	if provisioner.masterID != "master-001" {
		t.Fatalf("provisioned master = %q, want master-001", provisioner.masterID)
	}
}

func TestEnrollMasterReturnsUnavailableWithoutProvisioner(t *testing.T) {
	t.Parallel()
	service := New(&fakeRepository{}, nil)
	service.newToken = func() (string, error) { return "12345678901234567890123456789012", nil }

	_, err := service.EnrollMaster(t.Context(), "master-001", "12345678901234567890123456789012")
	if !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("EnrollMaster() error = %v, want %v", err, ErrCredentialUnavailable)
	}
}

type fakeRepository struct{}

func (fakeRepository) CreateMaster(context.Context, string, string, []byte) error { return nil }

func (fakeRepository) EnrollMaster(ctx context.Context, _ string, _ []byte, _ string, provision func(context.Context) error) error {
	return provision(ctx)
}

func (fakeRepository) EnrollSlave(context.Context, string, string, string, []byte) (SlaveEnrollment, error) {
	return SlaveEnrollment{}, nil
}

type fakeProvisioner struct {
	masterID string
}

func (p *fakeProvisioner) ProvisionMaster(_ context.Context, masterID, _ string) error {
	p.masterID = masterID
	return nil
}
