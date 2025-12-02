package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"agones.dev/agones/pkg/util/signals"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "agones.dev/agones/pkg/sdk"
)

func main() {
	// Get configuration from environment
	healthCheckType := getEnv("HEALTH_CHECK_TYPE", "port")
	port := getEnv("HEALTH_CHECK_PORT", "25565")
	protocol := getEnv("HEALTH_CHECK_PROTOCOL", "TCP")
	initialDelayStr := getEnv("HEALTH_CHECK_INITIAL_DELAY", "10")
	timeoutStr := getEnv("HEALTH_CHECK_TIMEOUT", "30")

	initialDelay, _ := strconv.Atoi(initialDelayStr)
	timeout, _ := strconv.Atoi(timeoutStr)

	log.Printf("Agones Sidecar Starting")
	log.Printf("  Health Check Type: %s", healthCheckType)
	log.Printf("  Port: %s, Protocol: %s", port, protocol)
	log.Printf("  Initial Delay: %ds, Timeout: %ds", initialDelay, timeout)

	ctx, cancel := signals.NewSigKillContext()
	defer cancel()

	// Setup graceful shutdown handler
	shutdownChan := make(chan struct{})
	signals.NewSigTermHandler(func() {
		log.Println("Received shutdown signal")
		close(shutdownChan)
	})

	// Wait for initial delay to let game server start up
	if initialDelay > 0 {
		log.Printf("Waiting %d seconds for game server startup...", initialDelay)
		time.Sleep(time.Duration(initialDelay) * time.Second)
	}

	// Wait for server to be ready based on health check
	log.Printf("Starting health checks...")
	startTime := time.Now()
	timeoutDuration := time.Duration(timeout) * time.Second

	for {
		select {
		case <-shutdownChan:
			log.Println("Exiting due to shutdown signal")
			return
		case <-ctx.Done():
			log.Println("Context cancelled")
			return
		default:
		}

		if time.Since(startTime) > timeoutDuration {
			log.Fatalf("Timeout waiting for server readiness after %d seconds", timeout)
		}

		ready := false
		switch healthCheckType {
		case "port":
			ready = checkPortReady(port, protocol)
		case "delay":
			// For delay type, we just wait the specified time
			if time.Since(startTime) >= timeoutDuration {
				ready = true
			}
		default:
			log.Fatalf("Unknown health check type: %s", healthCheckType)
		}

		if ready {
			log.Println("Server is ready! Calling sdk.Ready()...")
			err := callSdkReady(ctx)
			if err != nil {
				log.Fatalf("Failed to call sdk.Ready(): %v", err)
			}
			log.Println("Successfully called sdk.Ready()")
			return
		}

		// Wait a bit before next check
		time.Sleep(1 * time.Second)
	}
}

// checkPortReady checks if a port is open and accepting connections
func checkPortReady(port, protocol string) bool {
	addr := net.JoinHostPort("localhost", port)
	var network string

	switch protocol {
	case "TCP":
		network = "tcp"
	case "UDP":
		network = "udp"
	default:
		log.Printf("Unknown protocol: %s, defaulting to TCP", protocol)
		network = "tcp"
	}

	// For UDP, we can't really "connect", so we'll just return true after checking
	if network == "udp" {
		// Simple UDP check: try to resolve and see if port is in use
		conn, err := net.DialTimeout(network, addr, 1*time.Second)
		if err == nil {
			conn.Close()
			return true
		}
		// UDP might not respond, so we'll be more lenient
		// In practice, if the server is running on the port, we assume it's ready
		return false
	}

	// TCP check: attempt connection
	conn, err := net.DialTimeout(network, addr, 1*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// callSdkReady calls the Agones SDK Ready() method via gRPC
func callSdkReady(ctx context.Context) error {
	// Connect to Agones SDK sidecar (default: localhost:59357)
	sdkHost := getEnv("AGONES_SDK_GRPC_HOST", "localhost")
	sdkPort := getEnv("AGONES_SDK_GRPC_PORT", "59357")
	sdkAddr := fmt.Sprintf("%s:%s", sdkHost, sdkPort)

	log.Printf("Connecting to Agones SDK at %s", sdkAddr)

	// Create gRPC connection
	conn, err := grpc.DialContext(
		ctx,
		sdkAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to Agones SDK: %w", err)
	}
	defer conn.Close()

	// Create SDK client
	sdkClient := pb.NewSDKClient(conn)

	// Call Ready() on the SDK
	log.Println("Sending Ready() call to Agones SDK...")
	_, err = sdkClient.Ready(ctx, &pb.Empty{})
	if err != nil {
		return fmt.Errorf("failed to call sdk.Ready(): %w", err)
	}

	log.Println("Successfully called sdk.Ready()")
	return nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
