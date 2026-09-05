// Package store owns PostgreSQL access for Host.
package store

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"

	"example.com/farmhost/internal/enrollment"
	"example.com/farmhost/internal/telemetry"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ enrollment.Repository = (*Store)(nil)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping pool: %w", err)
	}
	return pool, nil
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func StoreBatch(ctx context.Context, pool *pgxpool.Pool, masterID string, batch telemetry.Batch) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `INSERT INTO telemetry_batches (master_id, message_id, measured_at, firmware_version) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, masterID, batch.MessageID, batch.MeasuredAt, batch.FirmwareVersion)
	if err != nil {
		return fmt.Errorf("insert batch: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil
	}
	for _, reading := range batch.Readings {
		result, err = tx.Exec(ctx, `INSERT INTO measurements (slave_id, measured_at, ph, ec_ms_per_cm, light_lux, soil_moisture_percent, calibration_version, firmware_version) SELECT id, $3, $4, $5, $6, $7, $8, $9 FROM slave_nodes WHERE id = $1 AND master_id = $2 AND disabled_at IS NULL ON CONFLICT DO NOTHING`, reading.SlaveID, masterID, batch.MeasuredAt, reading.PH, reading.ECMSPerCM, reading.LightLux, reading.SoilMoisturePercent, reading.CalibrationVersion, reading.FirmwareVersion)
		if err != nil {
			return fmt.Errorf("insert reading %q: %w", reading.SlaveID, err)
		}
		if result.RowsAffected() == 0 {
			return fmt.Errorf("unregistered or duplicate slave %q", reading.SlaveID)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *Store) CreateMaster(ctx context.Context, masterID, nodeLabel string, enrollmentTokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO master_nodes (id, node_label, enrollment_token_hash) VALUES ($1, $2, $3)`, masterID, nodeLabel, enrollmentTokenHash)
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" {
			return enrollment.ErrMasterExists
		}
		return fmt.Errorf("create master: %w", err)
	}
	return nil
}

func (s *Store) EnrollMaster(ctx context.Context, masterID string, enrollmentTokenHash []byte, mqttUsername string, provision func(context.Context) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin master enrollment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		storedToken []byte
		enrolledAt  *string
		disabledAt  *string
	)
	err = tx.QueryRow(ctx, `SELECT enrollment_token_hash, enrolled_at::text, disabled_at::text FROM master_nodes WHERE id = $1 FOR UPDATE`, masterID).Scan(&storedToken, &enrolledAt, &disabledAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return enrollment.ErrMasterNotFound
		}
		return fmt.Errorf("lock master enrollment: %w", err)
	}
	if disabledAt != nil {
		return enrollment.ErrMasterDisabled
	}
	if enrolledAt != nil || len(storedToken) == 0 {
		return enrollment.ErrEnrollmentUsed
	}
	if subtle.ConstantTimeCompare(storedToken, enrollmentTokenHash) != 1 {
		return enrollment.ErrInvalidEnrollment
	}
	if err := provision(ctx); err != nil {
		return fmt.Errorf("provision MQTT credential: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE master_nodes SET credential_username = $2, enrolled_at = now(), enrollment_token_hash = NULL WHERE id = $1`, masterID, mqttUsername)
	if err != nil {
		return fmt.Errorf("complete master enrollment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit master enrollment: %w", err)
	}
	return nil
}

func (s *Store) EnrollSlave(ctx context.Context, slaveID, masterID, nodeLabel string, transferTokenHash []byte) (enrollment.SlaveEnrollment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return enrollment.SlaveEnrollment{}, fmt.Errorf("begin slave enrollment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var enrolled bool
	err = tx.QueryRow(ctx, `SELECT enrolled_at IS NOT NULL FROM master_nodes WHERE id = $1 AND disabled_at IS NULL`, masterID).Scan(&enrolled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return enrollment.SlaveEnrollment{}, enrollment.ErrMasterNotFound
		}
		return enrollment.SlaveEnrollment{}, fmt.Errorf("find target master: %w", err)
	}
	if !enrolled {
		return enrollment.SlaveEnrollment{}, enrollment.ErrMasterNotEnrolled
	}

	var (
		potID       string
		storedToken []byte
	)
	err = tx.QueryRow(ctx, `SELECT pot_id::text, transfer_token_hash FROM slave_nodes WHERE id = $1 FOR UPDATE`, slaveID).Scan(&potID, &storedToken)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO pots (node_label) VALUES ($1) RETURNING id::text`, nodeLabel).Scan(&potID)
		if err != nil {
			return enrollment.SlaveEnrollment{}, fmt.Errorf("create pot: %w", err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO slave_nodes (id, master_id, pot_id, node_label, transfer_token_hash) VALUES ($1, $2, $3::uuid, $4, $5)`, slaveID, masterID, potID, nodeLabel, transferTokenHash)
		if err != nil {
			return enrollment.SlaveEnrollment{}, fmt.Errorf("create slave: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return enrollment.SlaveEnrollment{}, fmt.Errorf("commit slave enrollment: %w", err)
		}
		return enrollment.SlaveEnrollment{SlaveID: slaveID, MasterID: masterID, PotID: potID, Created: true}, nil
	}
	if err != nil {
		return enrollment.SlaveEnrollment{}, fmt.Errorf("lock slave enrollment: %w", err)
	}
	if subtle.ConstantTimeCompare(storedToken, transferTokenHash) != 1 {
		return enrollment.SlaveEnrollment{}, enrollment.ErrInvalidTransfer
	}
	_, err = tx.Exec(ctx, `UPDATE slave_nodes SET master_id = $2, node_label = $3 WHERE id = $1`, slaveID, masterID, nodeLabel)
	if err != nil {
		return enrollment.SlaveEnrollment{}, fmt.Errorf("transfer slave: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return enrollment.SlaveEnrollment{}, fmt.Errorf("commit slave transfer: %w", err)
	}
	return enrollment.SlaveEnrollment{SlaveID: slaveID, MasterID: masterID, PotID: potID}, nil
}
