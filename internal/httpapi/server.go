// Package httpapi provides Host HTTP endpoints.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"example.com/farmhost/internal/access"
	"example.com/farmhost/internal/enrollment"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EnrollmentService interface {
	CreateMasterProvisioning(context.Context, string, string) (enrollment.MasterProvisioning, error)
	EnrollMaster(context.Context, string, string) (enrollment.MasterCredentials, error)
	EnrollSlave(context.Context, string, string, string, string) (enrollment.SlaveEnrollment, error)
}

type Server struct {
	pool       *pgxpool.Pool
	logger     *slog.Logger
	http       *http.Server
	enrollment EnrollmentService
	authorizer *access.Authorizer
}

func NewServer(pool *pgxpool.Pool, logger *slog.Logger, addr string, enrollmentService EnrollmentService, authorizer *access.Authorizer) *Server {
	s := &Server{pool: pool, logger: logger, enrollment: enrollmentService, authorizer: authorizer}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /v1/dashboard/measurements", s.dashboardMeasurements)
	mux.HandleFunc("POST /v1/admin/master-enrollments", s.createMasterProvisioning)
	mux.HandleFunc("POST /v1/device/master-enrollments", s.enrollMaster)
	mux.HandleFunc("POST /v1/device/slave-enrollments", s.enrollSlave)
	s.http = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return s
}

type createMasterRequest struct {
	MasterID  string `json:"master_id"`
	NodeLabel string `json:"node_label"`
}

type enrollMasterRequest struct {
	MasterID        string `json:"master_id"`
	EnrollmentToken string `json:"enrollment_token"`
}

type enrollSlaveRequest struct {
	SlaveID       string `json:"slave_id"`
	MasterID      string `json:"master_id"`
	NodeLabel     string `json:"node_label"`
	TransferToken string `json:"transfer_token"`
}

func (s *Server) Run() error { return s.http.ListenAndServe() }

func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		s.logger.Error("health check failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

type dashboardResponse struct {
	Plants []dashboardPlant `json:"plants"`
}

type dashboardPlant struct {
	ID       string             `json:"id"`
	Label    string             `json:"label"`
	Readings []dashboardReading `json:"readings"`
}

type dashboardReading struct {
	MeasuredAt          time.Time `json:"measured_at"`
	PH                  float64   `json:"ph"`
	ECMSPerCM           float64   `json:"ec_ms_per_cm"`
	LightLux            float64   `json:"light_lux"`
	SoilMoisturePercent float64   `json:"soil_moisture_percent"`
}

func (s *Server) dashboardMeasurements(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT p.id::text,
			COALESCE(NULLIF(p.node_label, ''), NULLIF(slave_nodes.node_label, ''), slave_nodes.id),
			measurements.measured_at,
			measurements.ph,
			measurements.ec_ms_per_cm,
			measurements.light_lux,
			measurements.soil_moisture_percent
		FROM measurements
		JOIN slave_nodes ON slave_nodes.id = measurements.slave_id
		JOIN pots p ON p.id = slave_nodes.pot_id
		WHERE measurements.measured_at >= now() - interval '30 days'
		ORDER BY p.id, measurements.measured_at`)
	if err != nil {
		s.logger.Error("query dashboard measurements", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "dashboard data is unavailable"})
		return
	}
	defer rows.Close()

	// Use empty arrays in the public contract so a newly provisioned farm is a
	// valid, renderable dashboard response rather than `{\"plants\":null}`.
	response := dashboardResponse{Plants: make([]dashboardPlant, 0)}
	byID := make(map[string]int)
	for rows.Next() {
		var (
			plantID string
			label   string
			reading dashboardReading
		)
		if err := rows.Scan(&plantID, &label, &reading.MeasuredAt, &reading.PH, &reading.ECMSPerCM, &reading.LightLux, &reading.SoilMoisturePercent); err != nil {
			s.logger.Error("scan dashboard measurement", "err", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "dashboard data is unavailable"})
			return
		}
		index, found := byID[plantID]
		if !found {
			response.Plants = append(response.Plants, dashboardPlant{ID: plantID, Label: label, Readings: make([]dashboardReading, 0)})
			index = len(response.Plants) - 1
			byID[plantID] = index
		}
		response.Plants[index].Readings = append(response.Plants[index].Readings, reading)
	}
	if err := rows.Err(); err != nil {
		s.logger.Error("iterate dashboard measurements", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "dashboard data is unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) createMasterProvisioning(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}
	var request createMasterRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := s.enrollment.CreateMasterProvisioning(r.Context(), request.MasterID, request.NodeLabel)
	if err != nil {
		writeEnrollmentError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) enrollMaster(w http.ResponseWriter, r *http.Request) {
	var request enrollMasterRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := s.enrollment.EnrollMaster(r.Context(), request.MasterID, request.EnrollmentToken)
	if err != nil {
		writeEnrollmentError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) enrollSlave(w http.ResponseWriter, r *http.Request) {
	var request enrollSlaveRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := s.enrollment.EnrollSlave(r.Context(), request.SlaveID, request.MasterID, request.NodeLabel, request.TransferToken)
	if err != nil {
		writeEnrollmentError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) authorizeAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.authorizer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "admin authentication is not configured"})
		return false
	}
	actor, err := s.authorizer.Authorize(r.Context(), r.Header.Get(access.AssertionHeader))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	s.logger.Info("authorize admin request", "actor", actor)
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return false
	}
	return true
}

func writeEnrollmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, enrollment.ErrMasterExists):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "master already exists"})
	case errors.Is(err, enrollment.ErrMasterNotFound), errors.Is(err, enrollment.ErrInvalidEnrollment), errors.Is(err, enrollment.ErrInvalidTransfer):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "enrollment was not authorized"})
	case errors.Is(err, enrollment.ErrEnrollmentUsed):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "enrollment token was already used"})
	case errors.Is(err, enrollment.ErrMasterDisabled), errors.Is(err, enrollment.ErrMasterNotEnrolled):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "master is unavailable"})
	case errors.Is(err, enrollment.ErrCredentialUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credential provisioning is unavailable"})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enrollment request was rejected"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
