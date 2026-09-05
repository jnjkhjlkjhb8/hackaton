package telemetry

import (
	"testing"
	"time"
)

func TestBatchValidate(t *testing.T) {
	tests := []struct {
		name    string
		batch   Batch
		wantErr bool
	}{
		{name: "valid", batch: validBatch()},
		{name: "invalid ph", batch: Batch{MessageID: "m", MeasuredAt: time.Now(), FirmwareVersion: "1", Readings: []Reading{{SlaveID: "s", PH: 15, CalibrationVersion: "1", FirmwareVersion: "1"}}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.batch.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func validBatch() Batch {
	return Batch{MessageID: "m", MeasuredAt: time.Now(), FirmwareVersion: "1", Readings: []Reading{{SlaveID: "s", PH: 6.5, ECMSPerCM: 1.2, LightLux: 20, SoilMoisturePercent: 40, CalibrationVersion: "1", FirmwareVersion: "1"}}}
}
