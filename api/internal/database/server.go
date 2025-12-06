package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/internal/models"
)

type CreateServerParams struct {
	UserID               uuid.UUID
	DisplayName          string
	Subdomain            string
	Game                 models.GameType
	Plan                 models.ServerPlan
	StripeSubscriptionID *string
}

// CreateServer inserts a new server with pending status and populates the server model
func (db *DB) CreateServer(ctx context.Context, serverParams *CreateServerParams) (*models.Server, error) {
	query := `
		INSERT INTO servers (
			user_id, display_name, subdomain, game, plan, stripe_subscription_id
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, display_name, subdomain, game, plan, status, status_message,
		          node_ip, pod_ip, creation_error, last_reconciled, stripe_subscription_id,
		          created_at, updated_at, stopped_at, expired_at, delete_after
	`

	var server models.Server
	err := db.Pool.QueryRow(ctx, query,
		serverParams.UserID,
		serverParams.DisplayName,
		serverParams.Subdomain,
		serverParams.Game,
		serverParams.Plan,
		serverParams.StripeSubscriptionID,
	).Scan(
		&server.ID,
		&server.UserID,
		&server.DisplayName,
		&server.Subdomain,
		&server.Game,
		&server.Plan,
		&server.Status,
		&server.StatusMessage,
		&server.NodeIP,
		&server.PodIP,
		&server.CreationError,
		&server.LastReconciled,
		&server.StripeSubscriptionID,
		&server.CreatedAt,
		&server.UpdatedAt,
		&server.StoppedAt,
		&server.ExpiredAt,
		&server.DeleteAfter,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	return &server, nil
}

// GetServerByID retrieves a single server by ID
func (db *DB) GetServerByID(ctx context.Context, id string) (*models.Server, error) {
	query := `
		SELECT id, user_id, display_name, subdomain, game, plan, status, status_message,
		       node_ip, pod_ip, creation_error, last_reconciled, stripe_subscription_id,
		       created_at, updated_at, stopped_at, expired_at, delete_after
		FROM servers
		WHERE id = $1
	`

	var server models.Server
	err := db.Pool.QueryRow(ctx, query, id).Scan(
		&server.ID,
		&server.UserID,
		&server.DisplayName,
		&server.Subdomain,
		&server.Game,
		&server.Plan,
		&server.Status,
		&server.StatusMessage,
		&server.NodeIP,
		&server.PodIP,
		&server.CreationError,
		&server.LastReconciled,
		&server.StripeSubscriptionID,
		&server.CreatedAt,
		&server.UpdatedAt,
		&server.StoppedAt,
		&server.ExpiredAt,
		&server.DeleteAfter,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	return &server, nil
}

// GetServerWithDetails retrieves server with ports and volumes in a single query
func (db *DB) GetServerByIDWithDetails(ctx context.Context, id string) (*models.Server, error) {
	query := `
		SELECT
			s.id, s.user_id, s.display_name, s.subdomain, s.game, s.plan, s.status, s.status_message,
			s.node_ip, s.pod_ip, s.creation_error, s.last_reconciled, s.stripe_subscription_id,
			s.created_at, s.updated_at, s.stopped_at, s.expired_at, s.delete_after,
			COALESCE(
				(SELECT json_agg(json_build_object(
					'id', pa.id,
					'server_id', pa.server_id,
					'name', pa.port_name,
					'container_port', 0,
					'host_port', pa.port,
					'protocol', pa.protocol,
					'created_at', pa.created_at
				) ORDER BY pa.port_name)
				FROM port_allocations pa
				WHERE pa.server_id = s.id),
				'[]'::json
			) as ports,
			COALESCE(
				(SELECT json_agg(json_build_object(
					'id', v.id,
					'server_id', v.server_id,
					'name', v.name,
					'mount_path', v.mount_path,
					'sub_path', v.sub_path,
					'created_at', v.created_at
				) ORDER BY v.name)
				FROM server_volumes v
				WHERE v.server_id = s.id),
				'[]'::json
			) as volumes
		FROM servers s
		WHERE s.id = $1
	`

	var server models.Server
	var portsJSON, volumesJSON []byte

	err := db.Pool.QueryRow(ctx, query, id).Scan(
		&server.ID,
		&server.UserID,
		&server.DisplayName,
		&server.Subdomain,
		&server.Game,
		&server.Plan,
		&server.Status,
		&server.StatusMessage,
		&server.NodeIP,
		&server.PodIP,
		&server.CreationError,
		&server.LastReconciled,
		&server.StripeSubscriptionID,
		&server.CreatedAt,
		&server.UpdatedAt,
		&server.StoppedAt,
		&server.ExpiredAt,
		&server.DeleteAfter,
		&portsJSON,
		&volumesJSON,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get server with details: %w", err)
	}

	// Unmarshal JSON arrays into structs
	if err := json.Unmarshal(portsJSON, &server.Ports); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ports: %w", err)
	}

	if err := json.Unmarshal(volumesJSON, &server.Volumes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal volumes: %w", err)
	}

	return &server, nil
}

// ListServersByUser returns all servers for a user
func (db *DB) ListServersByUser(ctx context.Context, userID uuid.UUID) ([]models.Server, error) {
	query := `
		SELECT id, user_id, display_name, subdomain, game, plan, status, status_message,
		       node_ip, pod_ip, creation_error, last_reconciled, stripe_subscription_id,
		       created_at, updated_at, stopped_at, expired_at, delete_after
		FROM servers
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		var server models.Server
		err := rows.Scan(
			&server.ID,
			&server.UserID,
			&server.DisplayName,
			&server.Subdomain,
			&server.Game,
			&server.Plan,
			&server.Status,
			&server.StatusMessage,
			&server.NodeIP,
			&server.PodIP,
			&server.CreationError,
			&server.LastReconciled,
			&server.StripeSubscriptionID,
			&server.CreatedAt,
			&server.UpdatedAt,
			&server.StoppedAt,
			&server.ExpiredAt,
			&server.DeleteAfter,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan server: %w", err)
		}
		servers = append(servers, server)
	}

	return servers, nil
}

// GetAllServers returns all servers (for reconciler)
// Excludes hard-deleted servers (status != 'deleted' OR delete_after in future)
func (db *DB) GetAllServers(ctx context.Context) ([]models.Server, error) {
	query := `
		SELECT id, user_id, display_name, subdomain, game, plan, status, status_message,
		       node_ip, pod_ip, creation_error, last_reconciled, stripe_subscription_id,
		       created_at, updated_at, stopped_at, expired_at, delete_after
		FROM servers
		WHERE status != 'deleted' OR delete_after > NOW()
		ORDER BY created_at DESC
	`

	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all servers: %w", err)
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		var server models.Server
		err := rows.Scan(
			&server.ID,
			&server.UserID,
			&server.DisplayName,
			&server.Subdomain,
			&server.Game,
			&server.Plan,
			&server.Status,
			&server.StatusMessage,
			&server.NodeIP,
			&server.PodIP,
			&server.CreationError,
			&server.LastReconciled,
			&server.StripeSubscriptionID,
			&server.CreatedAt,
			&server.UpdatedAt,
			&server.StoppedAt,
			&server.ExpiredAt,
			&server.DeleteAfter,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan server: %w", err)
		}
		servers = append(servers, server)
	}

	return servers, nil
}

// UpdateServerStatus updates status and optional message
func (db *DB) UpdateServerStatus(ctx context.Context, id, status, message string) error {
	query := `
		UPDATE servers
		SET status = $2,
		    status_message = $3,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := db.Pool.Exec(ctx, query, id, status, message)
	if err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	return nil
}

// UpdateServerInfo updates IP and port (used by reconciler)
func (db *DB) UpdateServerInfo(ctx context.Context, id, nodeIP string) error {
	query := `
        UPDATE servers
        SET node_ip = $2,
            updated_at = NOW()
        WHERE id = $1
    `
	_, err := db.Pool.Exec(ctx, query, id, nodeIP)
	return err
}

// UpdateServerToRunning transitions server to running with full info
func (db *DB) UpdateServerToRunning(ctx context.Context, id, nodeIP string) error {
	query := `
        UPDATE servers
        SET status = 'running',
            status_message = NULL,
            node_ip = $2,
            updated_at = NOW()
        WHERE id = $1
    `
	_, err := db.Pool.Exec(ctx, query, id, nodeIP)
	return err
}

// UpdateServerPodIP updates the pod IP and marks reconciliation as complete
func (db *DB) UpdateServerPodIP(ctx context.Context, id, podIP string) error {
	query := `
		UPDATE servers
		SET pod_ip = $2,
		    last_reconciled = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := db.Pool.Exec(ctx, query, id, podIP)
	if err != nil {
		return fmt.Errorf("failed to update server pod IP: %w", err)
	}
	return nil
}

// MarkServerFailed marks a server as failed with an error message
func (db *DB) MarkServerFailed(ctx context.Context, id, errorMsg string) error {
	query := `
		UPDATE servers
		SET status = 'failed',
		    creation_error = $2,
		    last_reconciled = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := db.Pool.Exec(ctx, query, id, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to mark server as failed: %w", err)
	}
	return nil
}

// UpdateServerLastReconciled updates the last_reconciled timestamp
func (db *DB) UpdateServerLastReconciled(ctx context.Context, id string) error {
	query := `
		UPDATE servers
		SET last_reconciled = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update last_reconciled: %w", err)
	}
	return nil
}

// MarkServerStopped sets status to stopped
func (db *DB) MarkServerStopped(ctx context.Context, id string) error {
	query := `
		UPDATE servers
		SET status = 'stopped',
		    stopped_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark server stopped: %w", err)
	}

	return nil
}

// MarkServerDeleted marks server for deletion
func (db *DB) MarkServerDeleted(ctx context.Context, id string) error {
	query := `
		UPDATE servers
		SET status = 'deleted',
		    delete_after = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark server deleted: %w", err)
	}

	return nil
}

// HardDeleteServer permanently removes server from DB
func (db *DB) HardDeleteServer(ctx context.Context, id string) error {
	query := `DELETE FROM servers WHERE id = $1`

	_, err := db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to hard delete server: %w", err)
	}

	return nil
}

// UpdateServerNodeIP updates the node IP where server is running
func (db *DB) UpdateServerNodeIP(ctx context.Context, serverID, nodeIP string) error {
	query := `
        UPDATE servers
        SET node_ip = $2, updated_at = CURRENT_TIMESTAMP
        WHERE id = $1
    `
	_, err := db.Pool.Exec(ctx, query, serverID, nodeIP)
	if err != nil {
		return fmt.Errorf("failed to update server node IP: %w", err)
	}
	return nil
}

// CreateServerVolume inserts a volume configuration
func (db *DB) CreateServerVolume(ctx context.Context, vol *models.ServerVolume) error {
	query := `
        INSERT INTO server_volumes (server_id, name, mount_path, sub_path)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at
    `
	return db.Pool.QueryRow(ctx, query,
		vol.ServerID, vol.Name, vol.MountPath, vol.SubPath,
	).Scan(&vol.ID, &vol.CreatedAt)
}

// GetServerVolumes retrieves all volumes for a server
func (db *DB) GetServerVolumes(ctx context.Context, serverID string) ([]models.ServerVolume, error) {
	query := `
        SELECT id, server_id, name, mount_path, sub_path, created_at
        FROM server_volumes
        WHERE server_id = $1
        ORDER BY name
    `

	rows, err := db.Pool.Query(ctx, query, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server volumes: %w", err)
	}
	defer rows.Close()

	var volumes []models.ServerVolume
	for rows.Next() {
		var vol models.ServerVolume
		err := rows.Scan(
			&vol.ID,
			&vol.ServerID,
			&vol.Name,
			&vol.MountPath,
			&vol.SubPath,
			&vol.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan server volume: %w", err)
		}
		volumes = append(volumes, vol)
	}

	return volumes, nil
}

// GetServerByStripeSubscriptionID retrieves a server by its Stripe subscription ID
func (db *DB) GetServerByStripeSubscriptionID(ctx context.Context, subscriptionID string) (*models.Server, error) {
	query := `
		SELECT id, user_id, display_name, subdomain, game, plan, status, status_message,
		       node_ip, stripe_subscription_id,
		       created_at, updated_at, stopped_at, expired_at, delete_after
		FROM servers
		WHERE stripe_subscription_id = $1
	`

	var server models.Server
	err := db.Pool.QueryRow(ctx, query, subscriptionID).Scan(
		&server.ID,
		&server.UserID,
		&server.DisplayName,
		&server.Subdomain,
		&server.Game,
		&server.Plan,
		&server.Status,
		&server.StatusMessage,
		&server.NodeIP,
		&server.StripeSubscriptionID,
		&server.CreatedAt,
		&server.UpdatedAt,
		&server.StoppedAt,
		&server.ExpiredAt,
		&server.DeleteAfter,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get server by stripe subscription: %w", err)
	}

	return &server, nil
}

// MarkServerExpired marks a server as expired due to subscription end
// Clears node_name and resource reservations since ports are released separately
// PVC remains for the 7-day grace period
func (db *DB) MarkServerExpired(ctx context.Context, id string) error {
	query := `
		UPDATE servers
		SET status = 'expired',
		    expired_at = NOW(),
		    delete_after = NOW() + interval '7 days',
		    node_name = NULL,
		    reserved_cpu_millicores = NULL,
		    reserved_memory_bytes = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark server expired: %w", err)
	}

	return nil
}

// GetExpiredServersForCleanup retrieves servers that are expired and past their delete_after time
func (db *DB) GetExpiredServersForCleanup(ctx context.Context) ([]models.Server, error) {
	query := `
		SELECT id, user_id, display_name, subdomain, game, plan, status, status_message,
		       node_ip, pod_ip, creation_error, last_reconciled, stripe_subscription_id,
		       created_at, updated_at, stopped_at, expired_at, delete_after
		FROM servers
		WHERE delete_after <= NOW() AND status = 'expired'
		ORDER BY delete_after ASC
	`

	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired servers for cleanup: %w", err)
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		var server models.Server
		err := rows.Scan(
			&server.ID,
			&server.UserID,
			&server.DisplayName,
			&server.Subdomain,
			&server.Game,
			&server.Plan,
			&server.Status,
			&server.StatusMessage,
			&server.NodeIP,
			&server.PodIP,
			&server.CreationError,
			&server.LastReconciled,
			&server.StripeSubscriptionID,
			&server.CreatedAt,
			&server.UpdatedAt,
			&server.StoppedAt,
			&server.ExpiredAt,
			&server.DeleteAfter,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan server: %w", err)
		}
		servers = append(servers, server)
	}

	return servers, nil
}

// GetServersByStatus retrieves all servers with a given status (used by reconciler)
func (db *DB) GetServersByStatus(ctx context.Context, status string) ([]models.Server, error) {
	query := `
		SELECT id, user_id, display_name, subdomain, game, plan, status, status_message,
		       node_ip, pod_ip, creation_error, last_reconciled, stripe_subscription_id,
		       created_at, updated_at, stopped_at, expired_at, delete_after
		FROM servers
		WHERE status = $1
		ORDER BY last_reconciled ASC NULLS FIRST
	`

	rows, err := db.Pool.Query(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get servers by status: %w", err)
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		var server models.Server
		err := rows.Scan(
			&server.ID,
			&server.UserID,
			&server.DisplayName,
			&server.Subdomain,
			&server.Game,
			&server.Plan,
			&server.Status,
			&server.StatusMessage,
			&server.NodeIP,
			&server.PodIP,
			&server.CreationError,
			&server.LastReconciled,
			&server.StripeSubscriptionID,
			&server.CreatedAt,
			&server.UpdatedAt,
			&server.StoppedAt,
			&server.ExpiredAt,
			&server.DeleteAfter,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan server: %w", err)
		}
		servers = append(servers, server)
	}

	return servers, nil
}
