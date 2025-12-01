package k8s

import (
	"context"
	"fmt"
	"time"

	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	agonesclient "agones.dev/agones/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// PortConfig defines a port for GameServer
type PortConfig struct {
	Name          string
	ContainerPort int32
	Protocol      corev1.Protocol
}

// VolumeConfig defines a volume mount
type VolumeConfig struct {
	Name      string
	MountPath string
	SubPath   string
}

// Client wraps Kubernetes and Agones clients
type Client struct {
	clientset       *kubernetes.Clientset   // Standard K8s resources (Pods, PVCs, Nodes)
	agonesClientset *agonesclient.Clientset // Agones GameServers
	config          *rest.Config
}

// NewClient initializes a new Kubernetes client with in-cluster config or kubeconfig fallback
func NewClient() (*Client, error) {
	// Try in-cluster config first (when running in K8s)
	// This reads the ServiceAccount token from /var/run/secrets/kubernetes.io/serviceaccount/token
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig (for local development)
		// This reads from ~/.kube/config
		config, err = clientcmd.BuildConfigFromFlags("",
			clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
		}
	}

	// Create standard K8s client for core resources
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create K8s client: %w", err)
	}

	// Create Agones client for GameServer resources
	agonesClientset, err := agonesclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Agones client: %w", err)
	}

	return &Client{
		clientset:       clientset,
		agonesClientset: agonesClientset,
		config:          config,
	}, nil
}

// Health checks connectivity to the Kubernetes API server
func (c *Client) Health(ctx context.Context) error {
	_, err := c.clientset.Discovery().ServerVersion()
	return err
}

// CreatePVC creates a PersistentVolumeClaim for game data
func (c *Client) CreatePVC(ctx context.Context, namespace, name, storageSize string, labels map[string]string) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		},
	}

	_, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PVC: %w", err)
	}

	return nil
}

// CreateGameServer creates an Agones GameServer resource
func (c *Client) CreateGameServer(
	ctx context.Context,
	namespace, name, image string,
	ports []PortConfig,
	volumes []VolumeConfig,
	env map[string]string,
	cpuRequest, memoryRequest string,
	pvcName string,
	labels map[string]string,
) error {

	// Build environment variables
	var envVars []corev1.EnvVar
	for key, value := range env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	// Build Agones ports from multiple ports
	var gameServerPorts []agonesv1.GameServerPort
	for _, port := range ports {
		gameServerPorts = append(gameServerPorts, agonesv1.GameServerPort{
			Name:          port.Name,
			PortPolicy:    agonesv1.Dynamic,
			ContainerPort: port.ContainerPort,
			Protocol:      port.Protocol,
		})
	}

	// Build volume mounts from multiple volumes (all using same PVC)
	var volumeMounts []corev1.VolumeMount
	for _, vol := range volumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "server-data", // Single volume name
			MountPath: vol.MountPath,
			SubPath:   vol.SubPath, // Different subdirectories
		})
	}

	// Single PVC volume (all mounts reference this)
	podVolumes := []corev1.Volume{
		{
			Name: "server-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}

	gs := &agonesv1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: agonesv1.GameServerSpec{
			Ports: gameServerPorts, // Multiple ports
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "agones-sdk",
					Containers: []corev1.Container{
						{
							Name:         "game",
							Image:        image,
							Env:          envVars,
							VolumeMounts: volumeMounts, // Multiple mounts
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuRequest),
									corev1.ResourceMemory: resource.MustParse(memoryRequest),
								},
							},
						},
					},
					Volumes: podVolumes, // Single PVC
				},
			},
		},
	}

	_, err := c.agonesClientset.AgonesV1().GameServers(namespace).Create(ctx, gs, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create GameServer: %w", err)
	}

	return nil
}

// GetGameServer retrieves a single GameServer
func (c *Client) GetGameServer(ctx context.Context, namespace, name string) (*agonesv1.GameServer, error) {
	gs, err := c.agonesClientset.AgonesV1().GameServers(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get GameServer: %w", err)
	}
	return gs, nil
}

// ListGameServers lists all GameServers in a namespace
func (c *Client) ListGameServers(ctx context.Context, namespace string) ([]agonesv1.GameServer, error) {
	list, err := c.agonesClientset.AgonesV1().GameServers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list GameServers: %w", err)
	}
	return list.Items, nil
}

// DeleteGameServer deletes a GameServer
func (c *Client) DeleteGameServer(ctx context.Context, namespace, name string) error {
	err := c.agonesClientset.AgonesV1().GameServers(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete GameServer: %w", err)
	}
	return nil
}

// DeletePVC deletes a PersistentVolumeClaim
func (c *Client) DeletePVC(ctx context.Context, namespace, name string) error {
	err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete PVC: %w", err)
	}
	return nil
}

// GetNode retrieves a node by name
func (c *Client) GetNode(ctx context.Context, name string) (*corev1.Node, error) {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	return node, nil
}

// ListNodes lists all nodes in the cluster
func (c *Client) ListNodes(ctx context.Context) ([]corev1.Node, error) {
	list, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	return list.Items, nil
}

// WaitForGameServerReady polls GameServer until Ready state or timeout
func (c *Client) WaitForGameServerReady(ctx context.Context, namespace, name string, timeout time.Duration) (*agonesv1.GameServer, error) {
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for GameServer to be ready")
		}

		gs, err := c.GetGameServer(ctx, namespace, name)
		if err != nil {
			return nil, err
		}

		if gs.Status.State == agonesv1.GameServerStateReady {
			return gs, nil
		}

		// Wait before next check
		time.Sleep(5 * time.Second)
	}
}
