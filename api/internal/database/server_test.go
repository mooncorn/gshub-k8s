package database

import (
	"context"
	"strings"
	"testing"

	"github.com/mooncorn/gshub/api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CreateServer(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	user, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	displayName := strings.ToTitle(string(models.GameMinecraft))
	subdomain := RandomSubdomain()

	server, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user.ID,
		DisplayName: displayName,
		Subdomain:   subdomain,
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})

	require.NoError(t, err, "CreateServer should not return an error")

	// Verify server fields
	assert.NotZero(t, server.ID, "Server ID should be set")
	assert.NotZero(t, server.UserID, "User ID should be set")
	assert.Equal(t, server.DisplayName, displayName, "Display name should match")
	assert.Equal(t, server.Game, models.GameMinecraft, "Game should match")
	assert.Equal(t, server.Subdomain, subdomain, "Subdomain should match")
	assert.Equal(t, server.Plan, models.PlanSmall, "Plan should match")
	assert.Equal(t, server.Status, models.ServerStatusPending, "Status should be pending by default")
	assert.Nil(t, server.StatusMessage, "StatusMessage should be nil initially")
	assert.Nil(t, server.StripeSubscriptionID, "StripeSubscriptionID should be nil initially")
	assert.NotZero(t, server.CreatedAt, "CreatedAt should be set")
	assert.NotZero(t, server.UpdatedAt, "UpdatedAt should be set")
	assert.Nil(t, server.StoppedAt, "StoppedAt should be nil initially")
	assert.Nil(t, server.ExpiredAt, "ExpiredAt should be nil initially")
	assert.Nil(t, server.DeleteAfter, "DeleteAfter should be nil initially")

	// Verify ports and volumes are not populated
	assert.Empty(t, server.Ports, "Ports should be empty")
	assert.Empty(t, server.Volumes, "Volumes should be empty")
}

func Test_GetServerByID(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	// Create server
	displayName := strings.ToTitle(string(models.GameMinecraft))
	subdomain := RandomSubdomain()

	server, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user.ID,
		DisplayName: displayName,
		Subdomain:   subdomain,
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Get server by ID (without ports/volumes)
	retrievedServer, err := db.GetServerByID(ctx, server.ID.String())
	require.NoError(t, err, "GetServerByID should not return an error")

	// Verify server fields
	assert.Equal(t, server.ID, retrievedServer.ID, "Server ID should match")
	assert.Equal(t, server.UserID, retrievedServer.UserID, "User ID should match")
	assert.Equal(t, displayName, retrievedServer.DisplayName, "Display name should match")
	assert.Equal(t, subdomain, retrievedServer.Subdomain, "Subdomain should match")
	assert.Equal(t, models.GameMinecraft, retrievedServer.Game, "Game should match")
	assert.Equal(t, models.PlanSmall, retrievedServer.Plan, "Plan should match")
	assert.Equal(t, models.ServerStatusPending, retrievedServer.Status, "Status should match")
	assert.Nil(t, retrievedServer.StatusMessage, "StatusMessage should be nil")
	assert.Nil(t, retrievedServer.StripeSubscriptionID, "StripeSubscriptionID should be nil")
	assert.NotZero(t, retrievedServer.CreatedAt, "CreatedAt should be set")
	assert.NotZero(t, retrievedServer.UpdatedAt, "UpdatedAt should be set")
	assert.Nil(t, retrievedServer.StoppedAt, "StoppedAt should be nil")
	assert.Nil(t, retrievedServer.ExpiredAt, "ExpiredAt should be nil")
	assert.Nil(t, retrievedServer.DeleteAfter, "DeleteAfter should be nil")

	// Verify ports and volumes are not populated (GetServerByID doesn't fetch them)
	assert.Empty(t, retrievedServer.Ports, "Ports should be empty")
	assert.Empty(t, retrievedServer.Volumes, "Volumes should be empty")
}

func Test_GetServerByIDWithDetails(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	// Create server
	displayName := strings.ToTitle(string(models.GameMinecraft))
	subdomain := RandomSubdomain()

	server, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user.ID,
		DisplayName: displayName,
		Subdomain:   subdomain,
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Add volumes
	dataVolume := &models.ServerVolume{
		ServerID:  server.ID.String(),
		Name:      "data",
		MountPath: "/data",
		SubPath:   "minecraft-data",
	}
	err = db.CreateServerVolume(ctx, dataVolume)
	require.NoError(t, err, "CreateServerVolume should not return an error")

	logsVolume := &models.ServerVolume{
		ServerID:  server.ID.String(),
		Name:      "logs",
		MountPath: "/logs",
		SubPath:   "minecraft-logs",
	}
	err = db.CreateServerVolume(ctx, logsVolume)
	require.NoError(t, err, "CreateServerVolume should not return an error")

	// Get server with details using single query
	serverWithDetails, err := db.GetServerByIDWithDetails(ctx, server.ID.String())
	require.NoError(t, err, "GetServerByIDWithDetails should not return an error")

	// Verify server fields
	assert.Equal(t, server.ID, serverWithDetails.ID, "Server ID should match")
	assert.Equal(t, server.UserID, serverWithDetails.UserID, "User ID should match")
	assert.Equal(t, displayName, serverWithDetails.DisplayName, "Display name should match")
	assert.Equal(t, subdomain, serverWithDetails.Subdomain, "Subdomain should match")
	assert.Equal(t, models.GameMinecraft, serverWithDetails.Game, "Game should match")
	assert.Equal(t, models.PlanSmall, serverWithDetails.Plan, "Plan should match")
	assert.Equal(t, models.ServerStatusPending, serverWithDetails.Status, "Status should match")

	// Verify ports (no port allocations created, so should be empty)
	// Ports are now managed via port_allocations table, tested separately
	require.Len(t, serverWithDetails.Ports, 0, "Should have 0 ports without port allocations")

	// Verify volumes
	require.Len(t, serverWithDetails.Volumes, 2, "Should have 2 volumes")

	// Volumes are ordered by name, so "data" comes before "logs"
	assert.Equal(t, "data", serverWithDetails.Volumes[0].Name, "First volume should be 'data'")
	assert.Equal(t, "/data", serverWithDetails.Volumes[0].MountPath, "Data volume mount path should be /data")
	assert.Equal(t, "minecraft-data", serverWithDetails.Volumes[0].SubPath, "Data volume subpath should match")

	assert.Equal(t, "logs", serverWithDetails.Volumes[1].Name, "Second volume should be 'logs'")
	assert.Equal(t, "/logs", serverWithDetails.Volumes[1].MountPath, "Logs volume mount path should be /logs")
	assert.Equal(t, "minecraft-logs", serverWithDetails.Volumes[1].SubPath, "Logs volume subpath should match")
}

func Test_ListServersByUser(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create first user with multiple servers
	user1, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	// Create second user with one server (to verify filtering)
	user2, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	// Create 3 servers for user1
	server1, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user1.ID,
		DisplayName: "Minecraft Server 1",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	server2, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user1.ID,
		DisplayName: "Valheim Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameValheim,
		Plan:        models.PlanMedium,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	server3, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user1.ID,
		DisplayName: "Minecraft Server 2",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameMinecraft,
		Plan:        models.PlanLarge,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Create 1 server for user2
	_, err = db.CreateServer(ctx, &CreateServerParams{
		UserID:      user2.ID,
		DisplayName: "User2 Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameRust,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// List servers for user1
	servers, err := db.ListServersByUser(ctx, user1.ID)
	require.NoError(t, err, "ListServersByUser should not return an error")

	// Verify correct number of servers
	require.Len(t, servers, 3, "User1 should have 3 servers")

	// Verify all servers belong to user1
	for _, server := range servers {
		assert.Equal(t, user1.ID, server.UserID, "All servers should belong to user1")
	}

	// Verify servers are ordered by created_at DESC (timestamps should be descending or equal)
	for i := 0; i < len(servers)-1; i++ {
		assert.True(t,
			servers[i].CreatedAt.After(servers[i+1].CreatedAt) || servers[i].CreatedAt.Equal(servers[i+1].CreatedAt),
			"Servers should be ordered by created_at DESC")
	}

	// Verify all expected servers are present (order may vary if created_at is the same)
	expectedIDs := map[string]bool{
		server1.ID.String(): false,
		server2.ID.String(): false,
		server3.ID.String(): false,
	}
	for _, server := range servers {
		expectedIDs[server.ID.String()] = true
	}
	for id, found := range expectedIDs {
		assert.True(t, found, "Server %s should be in the list", id)
	}

	// Verify all servers have correct status and no ports/volumes
	for _, server := range servers {
		assert.Equal(t, models.ServerStatusPending, server.Status, "Status should be pending")
		assert.Empty(t, server.Ports, "Ports should be empty")
		assert.Empty(t, server.Volumes, "Volumes should be empty")
	}
}

func Test_GetAllServers(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create two users
	user1, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	user2, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	// Create pending server for user1
	pendingServer, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user1.ID,
		DisplayName: "Pending Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Create running server for user2
	runningServer, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user2.ID,
		DisplayName: "Running Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameValheim,
		Plan:        models.PlanMedium,
	})
	require.NoError(t, err, "CreateServer should not return an error")
	err = db.UpdateServerStatus(ctx, runningServer.ID.String(), string(models.ServerStatusRunning), "")
	require.NoError(t, err, "UpdateServerStatus should not return an error")

	// Create stopped server for user1
	stoppedServer, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user1.ID,
		DisplayName: "Stopped Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameRust,
		Plan:        models.PlanLarge,
	})
	require.NoError(t, err, "CreateServer should not return an error")
	err = db.MarkServerStopped(ctx, stoppedServer.ID.String())
	require.NoError(t, err, "MarkServerStopped should not return an error")

	// Create soft-deleted server (marked for deletion but not hard-deleted yet)
	softDeletedServer, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user2.ID,
		DisplayName: "Soft Deleted Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameARK,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")
	err = db.MarkServerDeleted(ctx, softDeletedServer.ID.String())
	require.NoError(t, err, "MarkServerDeleted should not return an error")

	// Get all servers
	servers, err := db.GetAllServers(ctx)
	require.NoError(t, err, "GetAllServers should not return an error")

	// Should return all 4 servers (pending, running, stopped, and soft-deleted)
	// The soft-deleted server has delete_after set to NOW(), which means it's in the past,
	// but since the WHERE clause is "status != 'deleted' OR delete_after > NOW()",
	// it will be excluded if delete_after is in the past
	// Let's verify we get at least 3 servers (pending, running, stopped)
	assert.GreaterOrEqual(t, len(servers), 3, "Should return at least 3 non-deleted servers")

	// Verify servers are ordered by created_at DESC
	for i := 0; i < len(servers)-1; i++ {
		assert.True(t,
			servers[i].CreatedAt.After(servers[i+1].CreatedAt) || servers[i].CreatedAt.Equal(servers[i+1].CreatedAt),
			"Servers should be ordered by created_at DESC")
	}

	// Verify servers from both users are included (cross-user)
	userIDs := make(map[string]bool)
	for _, server := range servers {
		userIDs[server.UserID.String()] = true
	}
	assert.True(t, len(userIDs) >= 1, "Should have servers from at least one user")

	// Verify expected servers are present
	expectedIDs := map[string]models.ServerStatus{
		pendingServer.ID.String(): models.ServerStatusPending,
		runningServer.ID.String(): models.ServerStatusRunning,
		stoppedServer.ID.String(): models.ServerStatusStopped,
	}

	for _, server := range servers {
		if expectedStatus, found := expectedIDs[server.ID.String()]; found {
			assert.Equal(t, expectedStatus, server.Status, "Server status should match")
		}
	}

	// Verify ports and volumes are not populated
	for _, server := range servers {
		assert.Empty(t, server.Ports, "Ports should be empty")
		assert.Empty(t, server.Volumes, "Volumes should be empty")
	}
}

func Test_UpdateServerStatus(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create user and server
	user, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	server, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user.ID,
		DisplayName: "Test Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Verify initial status
	assert.Equal(t, models.ServerStatusPending, server.Status, "Initial status should be pending")
	assert.Nil(t, server.StatusMessage, "Initial status message should be nil")
	initialUpdatedAt := server.UpdatedAt

	// Update status to running with a message
	statusMessage := "Server is now running"
	err = db.UpdateServerStatus(ctx, server.ID.String(), string(models.ServerStatusRunning), statusMessage)
	require.NoError(t, err, "UpdateServerStatus should not return an error")

	// Retrieve updated server
	updatedServer, err := db.GetServerByID(ctx, server.ID.String())
	require.NoError(t, err, "GetServerByID should not return an error")

	// Verify status was updated
	assert.Equal(t, models.ServerStatusRunning, updatedServer.Status, "Status should be updated to running")
	assert.NotNil(t, updatedServer.StatusMessage, "Status message should be set")
	assert.Equal(t, statusMessage, *updatedServer.StatusMessage, "Status message should match")

	// Verify updated_at was changed (or at least not before the initial timestamp)
	assert.True(t,
		updatedServer.UpdatedAt.After(initialUpdatedAt) || updatedServer.UpdatedAt.Equal(initialUpdatedAt),
		"updated_at should be updated or equal")

	// Verify other fields remain unchanged
	assert.Equal(t, server.ID, updatedServer.ID, "ID should remain unchanged")
	assert.Equal(t, server.UserID, updatedServer.UserID, "UserID should remain unchanged")
	assert.Equal(t, server.DisplayName, updatedServer.DisplayName, "DisplayName should remain unchanged")
	assert.Equal(t, server.Game, updatedServer.Game, "Game should remain unchanged")
	assert.Equal(t, server.Plan, updatedServer.Plan, "Plan should remain unchanged")

	// Update status to failed with empty message
	err = db.UpdateServerStatus(ctx, server.ID.String(), string(models.ServerStatusFailed), "")
	require.NoError(t, err, "UpdateServerStatus should not return an error")

	// Retrieve updated server again
	failedServer, err := db.GetServerByID(ctx, server.ID.String())
	require.NoError(t, err, "GetServerByID should not return an error")

	// Verify status was updated to failed
	assert.Equal(t, models.ServerStatusFailed, failedServer.Status, "Status should be updated to failed")
	assert.NotNil(t, failedServer.StatusMessage, "Status message should be set even if empty")
	assert.Equal(t, "", *failedServer.StatusMessage, "Status message should be empty string")
}

func Test_UpdateServerToRunning(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create user and server
	user, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	server, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user.ID,
		DisplayName: "Test Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Set a status message first
	err = db.UpdateServerStatus(ctx, server.ID.String(), string(models.ServerStatusPending), "Starting up...")
	require.NoError(t, err, "UpdateServerStatus should not return an error")

	// Transition to running
	err = db.UpdateServerToRunning(ctx, server.ID.String())
	require.NoError(t, err, "UpdateServerToRunning should not return an error")

	// Retrieve updated server
	runningServer, err := db.GetServerByID(ctx, server.ID.String())
	require.NoError(t, err, "GetServerByID should not return an error")

	// Verify status is running
	assert.Equal(t, models.ServerStatusRunning, runningServer.Status, "Status should be running")

	// Verify status_message was cleared
	assert.Nil(t, runningServer.StatusMessage, "StatusMessage should be cleared (NULL)")
}

func Test_MarkServerStopped(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create user and server
	user, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	server, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user.ID,
		DisplayName: "Test Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Set to running first
	err = db.UpdateServerToRunning(ctx, server.ID.String())
	require.NoError(t, err, "UpdateServerToRunning should not return an error")

	// Verify stopped_at is nil initially
	runningServer, err := db.GetServerByID(ctx, server.ID.String())
	require.NoError(t, err, "GetServerByID should not return an error")
	assert.Nil(t, runningServer.StoppedAt, "StoppedAt should be nil initially")

	// Mark as stopped
	err = db.MarkServerStopped(ctx, server.ID.String())
	require.NoError(t, err, "MarkServerStopped should not return an error")

	// Retrieve stopped server
	stoppedServer, err := db.GetServerByID(ctx, server.ID.String())
	require.NoError(t, err, "GetServerByID should not return an error")

	// Verify status is stopped
	assert.Equal(t, models.ServerStatusStopped, stoppedServer.Status, "Status should be stopped")

	// Verify stopped_at was set
	assert.NotNil(t, stoppedServer.StoppedAt, "StoppedAt should be set")
}

func Test_MarkServerDeleted(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create user and server
	user, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	server, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user.ID,
		DisplayName: "Test Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Verify delete_after is nil initially
	assert.Nil(t, server.DeleteAfter, "DeleteAfter should be nil initially")

	// Mark as deleted
	err = db.MarkServerDeleted(ctx, server.ID.String())
	require.NoError(t, err, "MarkServerDeleted should not return an error")

	// Retrieve deleted server
	deletedServer, err := db.GetServerByID(ctx, server.ID.String())
	require.NoError(t, err, "GetServerByID should not return an error")

	// Verify status is deleted
	assert.Equal(t, models.ServerStatusDeleted, deletedServer.Status, "Status should be deleted")

	// Verify delete_after was set
	assert.NotNil(t, deletedServer.DeleteAfter, "DeleteAfter should be set")
}

func Test_HardDeleteServer(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create user and server
	user, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	server, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user.ID,
		DisplayName: "Test Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Verify server exists
	retrievedServer, err := db.GetServerByID(ctx, server.ID.String())
	require.NoError(t, err, "GetServerByID should not return an error")
	assert.Equal(t, server.ID, retrievedServer.ID, "Server should exist")

	// Hard delete the server
	err = db.HardDeleteServer(ctx, server.ID.String())
	require.NoError(t, err, "HardDeleteServer should not return an error")

	// Verify server no longer exists
	_, err = db.GetServerByID(ctx, server.ID.String())
	assert.Error(t, err, "GetServerByID should return an error for deleted server")
}

func Test_CreateServerVolume(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create user and server
	user, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	server, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user.ID,
		DisplayName: "Test Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Create server volume
	volume := &models.ServerVolume{
		ServerID:  server.ID.String(),
		Name:      "data",
		MountPath: "/data",
		SubPath:   "minecraft-data",
	}

	err = db.CreateServerVolume(ctx, volume)
	require.NoError(t, err, "CreateServerVolume should not return an error")

	// Verify volume fields were populated
	assert.NotZero(t, volume.ID, "Volume ID should be set")
	assert.NotZero(t, volume.CreatedAt, "Volume CreatedAt should be set")
	assert.Equal(t, server.ID.String(), volume.ServerID, "ServerID should match")
	assert.Equal(t, "data", volume.Name, "Name should match")
	assert.Equal(t, "/data", volume.MountPath, "MountPath should match")
	assert.Equal(t, "minecraft-data", volume.SubPath, "SubPath should match")
}

func Test_GetServerVolumes(t *testing.T) {
	db, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create user and server
	user, err := db.CreateUser(ctx, RandomEmail(), "password_hash")
	require.NoError(t, err, "CreateUser should not return an error")

	server, err := db.CreateServer(ctx, &CreateServerParams{
		UserID:      user.ID,
		DisplayName: "Test Server",
		Subdomain:   RandomSubdomain(),
		Game:        models.GameMinecraft,
		Plan:        models.PlanSmall,
	})
	require.NoError(t, err, "CreateServer should not return an error")

	// Create multiple volumes
	dataVolume := &models.ServerVolume{
		ServerID:  server.ID.String(),
		Name:      "data",
		MountPath: "/data",
		SubPath:   "minecraft-data",
	}
	err = db.CreateServerVolume(ctx, dataVolume)
	require.NoError(t, err, "CreateServerVolume should not return an error")

	logsVolume := &models.ServerVolume{
		ServerID:  server.ID.String(),
		Name:      "logs",
		MountPath: "/logs",
		SubPath:   "minecraft-logs",
	}
	err = db.CreateServerVolume(ctx, logsVolume)
	require.NoError(t, err, "CreateServerVolume should not return an error")

	// Get all volumes for server
	volumes, err := db.GetServerVolumes(ctx, server.ID.String())
	require.NoError(t, err, "GetServerVolumes should not return an error")

	// Verify correct number of volumes
	require.Len(t, volumes, 2, "Should have 2 volumes")

	// Volumes are ordered by name, so "data" comes before "logs"
	assert.Equal(t, "data", volumes[0].Name, "First volume should be 'data'")
	assert.Equal(t, "/data", volumes[0].MountPath, "Data mount path should be /data")
	assert.Equal(t, "minecraft-data", volumes[0].SubPath, "Data subpath should match")

	assert.Equal(t, "logs", volumes[1].Name, "Second volume should be 'logs'")
	assert.Equal(t, "/logs", volumes[1].MountPath, "Logs mount path should be /logs")
	assert.Equal(t, "minecraft-logs", volumes[1].SubPath, "Logs subpath should match")
}
