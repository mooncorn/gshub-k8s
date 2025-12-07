package watcher

import (
	"context"
	"strings"
	"time"

	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	agonesclient "agones.dev/agones/pkg/client/clientset/versioned"
	agonesInformers "agones.dev/agones/pkg/client/informers/externalversions"
	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/models"
	"github.com/mooncorn/gshub/api/internal/services/broadcast"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
)

const (
	// serverNamePrefix is the prefix for GameServer names
	serverNamePrefix = "server-"
	// defaultResyncPeriod is how often the informer does a full resync
	defaultResyncPeriod = 12 * time.Hour
)

// Service watches GameServer resources and updates the database in real-time
type Service struct {
	db              *database.DB
	hub             *broadcast.Hub
	agonesClientset agonesclient.Clientset
	informerFactory agonesInformers.SharedInformerFactory
	namespace       string
	logger          *zap.Logger
	stopCh          chan struct{}
}

// NewService creates a new watcher service
func NewService(
	db *database.DB,
	agonesClientset *agonesclient.Clientset,
	hub *broadcast.Hub,
	logger *zap.Logger,
	namespace string,
) *Service {
	return &Service{
		db:              db,
		hub:             hub,
		agonesClientset: *agonesClientset,
		namespace:       namespace,
		logger:          logger.Named("watcher"),
		stopCh:          make(chan struct{}),
	}
}

// Start begins watching GameServer resources
func (s *Service) Start(ctx context.Context) {
	s.logger.Info("starting GameServer watcher",
		zap.String("namespace", s.namespace),
	)

	// Create informer factory for the namespace
	s.informerFactory = agonesInformers.NewSharedInformerFactoryWithOptions(
		&s.agonesClientset,
		defaultResyncPeriod,
		agonesInformers.WithNamespace(s.namespace),
	)

	// Get the GameServer informer
	gsInformer := s.informerFactory.Agones().V1().GameServers().Informer()

	// Add event handlers
	gsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    s.handleAdd,
		UpdateFunc: s.handleUpdate,
		DeleteFunc: s.handleDelete,
	})

	// Start the informer factory
	s.informerFactory.Start(s.stopCh)

	// Wait for cache sync
	s.logger.Info("waiting for informer cache sync")
	if !cache.WaitForCacheSync(s.stopCh, gsInformer.HasSynced) {
		s.logger.Error("failed to sync informer cache")
		return
	}
	s.logger.Info("informer cache synced successfully")
}

// Stop stops the watcher service
func (s *Service) Stop() {
	s.logger.Info("stopping GameServer watcher")
	close(s.stopCh)
}

// handleAdd handles GameServer add events
func (s *Service) handleAdd(obj interface{}) {
	gs, ok := obj.(*agonesv1.GameServer)
	if !ok {
		s.logger.Error("received non-GameServer object in add handler")
		return
	}

	s.processGameServerEvent(gs, "add")
}

// handleUpdate handles GameServer update events
func (s *Service) handleUpdate(oldObj, newObj interface{}) {
	oldGS, ok := oldObj.(*agonesv1.GameServer)
	if !ok {
		return
	}
	newGS, ok := newObj.(*agonesv1.GameServer)
	if !ok {
		return
	}

	// Only process if state changed
	if oldGS.Status.State == newGS.Status.State {
		return
	}

	s.processGameServerEvent(newGS, "update")
}

// handleDelete handles GameServer delete events
func (s *Service) handleDelete(obj interface{}) {
	gs, ok := obj.(*agonesv1.GameServer)
	if !ok {
		// Handle DeletedFinalStateUnknown
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			s.logger.Error("couldn't get object from tombstone")
			return
		}
		gs, ok = tombstone.Obj.(*agonesv1.GameServer)
		if !ok {
			s.logger.Error("tombstone contained non-GameServer object")
			return
		}
	}

	s.processGameServerDelete(gs)
}

// processGameServerEvent processes a GameServer state change
func (s *Service) processGameServerEvent(gs *agonesv1.GameServer, eventType string) {
	ctx := context.Background()

	// Extract server ID from GameServer name
	serverID, err := s.extractServerID(gs.Name)
	if err != nil {
		s.logger.Debug("skipping GameServer - not managed by gshub",
			zap.String("name", gs.Name),
		)
		return
	}

	s.logger.Debug("processing GameServer event",
		zap.String("server_id", serverID),
		zap.String("event_type", eventType),
		zap.String("gs_state", string(gs.Status.State)),
	)

	// Get server from DB to verify it exists and get user ID
	server, err := s.db.GetServerByID(ctx, serverID)
	if err != nil {
		s.logger.Warn("server not found in database",
			zap.String("server_id", serverID),
			zap.Error(err),
		)
		return
	}

	// Map GameServer state to DB status and perform transition
	var transitioned bool
	var newStatus models.ServerStatus

	switch gs.Status.State {
	case agonesv1.GameServerStateCreating,
		agonesv1.GameServerStateStarting,
		agonesv1.GameServerStateScheduled,
		agonesv1.GameServerStateRequestReady:
		// Transition pending -> starting
		transitioned, err = s.db.TransitionServerStatus(
			ctx, serverID,
			models.ServerStatusPending, models.ServerStatusStarting,
			"GameServer is starting",
		)
		newStatus = models.ServerStatusStarting

	case agonesv1.GameServerStateReady:
		// Transition starting -> running
		transitioned, err = s.db.TransitionServerStatus(
			ctx, serverID,
			models.ServerStatusStarting, models.ServerStatusRunning,
			"Server is running",
		)
		newStatus = models.ServerStatusRunning

	case agonesv1.GameServerStateShutdown:
		// Transition running -> stopping
		transitioned, err = s.db.TransitionServerStatus(
			ctx, serverID,
			models.ServerStatusRunning, models.ServerStatusStopping,
			"Server is shutting down",
		)
		newStatus = models.ServerStatusStopping

	default:
		// Unknown state, log and skip
		s.logger.Debug("unhandled GameServer state",
			zap.String("server_id", serverID),
			zap.String("state", string(gs.Status.State)),
		)
		return
	}

	if err != nil {
		s.logger.Error("failed to transition server status",
			zap.String("server_id", serverID),
			zap.Error(err),
		)
		return
	}

	if transitioned {
		s.logger.Info("server status transitioned",
			zap.String("server_id", serverID),
			zap.String("new_status", string(newStatus)),
		)

		// Publish to hub
		s.publishStatusEvent(server.UserID, serverID, newStatus)
	}
}

// processGameServerDelete handles GameServer deletion
func (s *Service) processGameServerDelete(gs *agonesv1.GameServer) {
	ctx := context.Background()

	serverID, err := s.extractServerID(gs.Name)
	if err != nil {
		return
	}

	s.logger.Debug("processing GameServer deletion",
		zap.String("server_id", serverID),
	)

	// Get server from DB
	server, err := s.db.GetServerByID(ctx, serverID)
	if err != nil {
		s.logger.Warn("server not found in database for deletion",
			zap.String("server_id", serverID),
			zap.Error(err),
		)
		return
	}

	// Transition stopping -> stopped
	transitioned, err := s.db.TransitionServerStatus(
		ctx, serverID,
		models.ServerStatusStopping, models.ServerStatusStopped,
		"Server stopped",
	)
	if err != nil {
		s.logger.Error("failed to transition server to stopped",
			zap.String("server_id", serverID),
			zap.Error(err),
		)
		return
	}

	if transitioned {
		// Mark server as stopped with timestamp
		if markErr := s.db.MarkServerStopped(ctx, serverID); markErr != nil {
			s.logger.Warn("failed to mark server stopped",
				zap.String("server_id", serverID),
				zap.Error(markErr),
			)
		}

		s.logger.Info("server stopped",
			zap.String("server_id", serverID),
		)

		s.publishStatusEvent(server.UserID, serverID, models.ServerStatusStopped)
	}
}

// extractServerID extracts the server UUID from a GameServer name
func (s *Service) extractServerID(name string) (string, error) {
	if !strings.HasPrefix(name, serverNamePrefix) {
		return "", nil
	}

	serverID := strings.TrimPrefix(name, serverNamePrefix)

	// Validate it's a valid UUID
	if _, err := uuid.Parse(serverID); err != nil {
		return "", err
	}

	return serverID, nil
}

// publishStatusEvent publishes a status change event to the hub
func (s *Service) publishStatusEvent(userID uuid.UUID, serverID string, status models.ServerStatus) {
	event := broadcast.StatusEvent{
		ServerID:  serverID,
		Status:    string(status),
		Timestamp: time.Now().UTC(),
	}

	s.hub.Publish(userID, event)
}
