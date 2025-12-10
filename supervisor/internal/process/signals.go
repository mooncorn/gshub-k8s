package process

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

// SignalHandler manages graceful shutdown on signals
type SignalHandler struct {
	manager *Manager
	logger  *zap.Logger
	sigCh   chan os.Signal
	doneCh  chan struct{}
}

// NewSignalHandler creates a new signal handler
func NewSignalHandler(manager *Manager, logger *zap.Logger) *SignalHandler {
	return &SignalHandler{
		manager: manager,
		logger:  logger,
		sigCh:   make(chan os.Signal, 1),
		doneCh:  make(chan struct{}),
	}
}

// Start begins listening for shutdown signals
func (h *SignalHandler) Start(ctx context.Context) {
	signal.Notify(h.sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		defer close(h.doneCh)

		select {
		case sig := <-h.sigCh:
			h.logger.Info("received shutdown signal", zap.String("signal", sig.String()))
			h.handleShutdown(ctx)
		case <-ctx.Done():
			h.logger.Info("context cancelled, initiating shutdown")
			h.handleShutdown(context.Background())
		}
	}()
}

// handleShutdown performs graceful shutdown
func (h *SignalHandler) handleShutdown(ctx context.Context) {
	if h.manager.IsRunning() {
		h.logger.Info("stopping game process gracefully")
		if err := h.manager.Stop(ctx, true); err != nil {
			h.logger.Error("error stopping process", zap.Error(err))
		}
	}
}

// Wait blocks until shutdown is complete
func (h *SignalHandler) Wait() {
	<-h.doneCh
}

// Stop stops listening for signals
func (h *SignalHandler) Stop() {
	signal.Stop(h.sigCh)
	close(h.sigCh)
}
