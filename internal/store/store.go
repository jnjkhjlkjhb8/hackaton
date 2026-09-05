// Package store owns PostgreSQL access for Host.
package store

import (
	"context"
	"fmt"

	"example.com/farmhost/internal/telemetry"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
