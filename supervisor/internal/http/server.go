package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mooncorn/gshub/supervisor/internal/process"
	"go.uber.org/zap"
)

// StatusResponse contains detailed status information
type StatusResponse struct {
	Healthy       bool   `json:"healthy"`
	ProcessStatus string `json:"process_status"`
	ProcessPID    int    `json:"process_pid"`
	Uptime        string `json:"uptime"`
	GameHealthy   bool   `json:"game_healthy"`
	Message       string `json:"message,omitempty"`
}

// ManagerInterface defines what we need from the process manager
type ManagerInterface interface {
	IsRunning() bool
	IsHealthy() bool
	Status() process.Status
	PID() int
}

// Server provides HTTP health endpoints for K8s probes
type Server struct {
	port       int
	manager    ManagerInterface
	logger     *zap.Logger
	httpServer *http.Server
	startTime  time.Time
}

// NewServer creates a new HTTP health server
func NewServer(port int, manager ManagerInterface, logger *zap.Logger) *Server {
	return &Server{
		port:      port,
		manager:   manager,
		logger:    logger,
		startTime: time.Now(),
	}
}

// Start begins serving HTTP requests
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleLiveness)
	mux.HandleFunc("/readyz", s.handleReadiness)
	mux.HandleFunc("/status", s.handleStatus)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	// Graceful shutdown when context is cancelled
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(shutdownCtx)
	}()

	s.logger.Info("starting health server", zap.Int("port", s.port))
	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("health server error: %w", err)
	}
	return nil
}

// handleLiveness responds to K8s liveness probes
// Returns 200 if supervisor process is alive
func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// handleReadiness responds to K8s readiness probes
// Returns 200 only if game process is healthy
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if s.manager.IsHealthy() && s.manager.IsRunning() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not ready"))
	}
}

// handleStatus returns detailed status information for debugging
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := StatusResponse{
		Healthy:       s.manager.IsRunning(),
		ProcessStatus: string(s.manager.Status()),
		ProcessPID:    s.manager.PID(),
		Uptime:        time.Since(s.startTime).Round(time.Second).String(),
		GameHealthy:   s.manager.IsHealthy(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
