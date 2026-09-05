// Package telemetry validates telemetry received from registered Master Nodes.
package telemetry

import (
	"errors"
	"fmt"
	"time"
)

const maxBatchReadings = 128

type Batch struct {
	MessageID       string    `json:"message_id"`
	MeasuredAt      time.Time `json:"measured_at"`
	FirmwareVersion string    `json:"firmware_version"`
	Readings        []Reading `json:"readings"`
}

type Reading struct {
	SlaveID             string  `json:"slave_id"`
	PH                  float64 `json:"ph"`
	ECMSPerCM           float64 `json:"ec_ms_per_cm"`
	LightLux            float64 `json:"light_lux"`
	SoilMoisturePercent float64 `json:"soil_moisture_percent"`
	CalibrationVersion  string  `json:"calibration_version"`
	FirmwareVersion     string  `json:"firmware_version"`
}

func (b Batch) Validate() error {
	if b.MessageID == "" {
		return errors.New("message_id is required")
	}
	if b.MeasuredAt.IsZero() {
		return errors.New("measured_at is required")
	}
	if b.FirmwareVersion == "" {
		return errors.New("firmware_version is required")
	}
	if len(b.Readings) == 0 {
		return errors.New("readings is required")
	}
	if len(b.Readings) > maxBatchReadings {
		return fmt.Errorf("too many readings: %d", len(b.Readings))
	}
	for _, reading := range b.Readings {
		if err := reading.Validate(); err != nil {
			return fmt.Errorf("reading %q: %w", reading.SlaveID, err)
		}
	}
	return nil
}

func (r Reading) Validate() error {
	if r.SlaveID == "" {
		return errors.New("slave_id is required")
	}
	if r.CalibrationVersion == "" {
		return errors.New("calibration_version is required")
	}
	if r.FirmwareVersion == "" {
		return errors.New("firmware_version is required")
	}
	if r.PH < 0 || r.PH > 14 {
		return errors.New("ph must be between 0 and 14")
	}
	if r.ECMSPerCM < 0 {
		return errors.New("ec_ms_per_cm must not be negative")
	}
	if r.LightLux < 0 {
		return errors.New("light_lux must not be negative")
	}
	if r.SoilMoisturePercent < 0 || r.SoilMoisturePercent > 100 {
		return errors.New("soil_moisture_percent must be between 0 and 100")
	}
	return nil
}
