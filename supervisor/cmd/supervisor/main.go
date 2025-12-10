package main

import (
	"context"
	"os"
	"time"

	"github.com/mooncorn/gshub/supervisor/internal/api"
	"github.com/mooncorn/gshub/supervisor/internal/config"
	supervisorhttp "github.com/mooncorn/gshub/supervisor/internal/http"
	"github.com/mooncorn/gshub/supervisor/internal/metrics"
	"github.com/mooncorn/gshub/supervisor/internal/process"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Initialize logger
	logConfig := zap.NewProductionConfig()
	logConfig.EncoderConfig.TimeKey = "timestamp"
	logConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := logConfig.Build()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("supervisor starting")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	logger.Info("configuration loaded",
		zap.String("server_id", cfg.ServerID),
		zap.String("api_endpoint", cfg.APIEndpoint),
		zap.Strings("start_command", cfg.StartCommand),
		zap.String("work_dir", cfg.WorkDir),
		zap.Duration("grace_period", cfg.GracePeriod),
		zap.String("health_type", cfg.HealthType))

	// Create context for the application
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize API client
	apiClient := api.NewClient(cfg.APIEndpoint, cfg.ServerID, cfg.AuthToken, logger)

	// Initialize process manager
	manager, err := process.NewManager(cfg, apiClient, logger)
	if err != nil {
		logger.Fatal("failed to create process manager", zap.Error(err))
	}

	// Start HTTP health server for K8s probes
	healthServer := supervisorhttp.NewServer(cfg.HealthServerPort, manager, logger)
	go func() {
		if err := healthServer.Start(ctx); err != nil {
			logger.Error("health server error", zap.Error(err))
		}
	}()

	// Set up signal handling
	signalHandler := process.NewSignalHandler(manager, logger)
	signalHandler.Start(ctx)

	// Start the game process
	if err := manager.Start(ctx); err != nil {
		logger.Error("failed to start game process", zap.Error(err))
		os.Exit(1)
	}

	// Start continuous health monitoring after startup
	go manager.StartContinuousHealthCheck(ctx, func(status, message string) {
		apiClient.ReportStatusWithRetry(ctx, api.Status(status), message, manager.PID(), 3)
	})

	// Start heartbeat loop
	go runHeartbeat(ctx, cfg, apiClient, manager, logger)

	// Wait for the process to exit (either from signal or crash)
	manager.Wait()

	// Wait for signal handler to complete (ensures status is reported before exit)
	signalHandler.Wait()

	// Clean up
	cancel()
	signalHandler.Stop()

	exitCode := manager.ExitCode()
	logger.Info("supervisor exiting", zap.Int("exit_code", exitCode))

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// runHeartbeat sends periodic heartbeats to the API
func runHeartbeat(ctx context.Context, cfg *config.Config, apiClient *api.Client, manager *process.Manager, logger *zap.Logger) {
	ticker := time.NewTicker(cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if manager.IsRunning() {
				pid := manager.PID()

				// Collect actual memory metrics from procfs
				memoryMB := int64(0)
				cpuPercent := float64(0)

				if processMetrics, err := metrics.CollectProcessMetrics(pid); err == nil {
					memoryMB = processMetrics.MemoryMB
					cpuPercent = processMetrics.CPUPercent
				}

				if err := apiClient.SendHeartbeat(ctx, pid, memoryMB, cpuPercent); err != nil {
					logger.Warn("failed to send heartbeat", zap.Error(err))
				} else {
					logger.Debug("heartbeat sent", zap.Int("pid", pid), zap.Int64("memory_mb", memoryMB))
				}
			}
		}
	}
}
