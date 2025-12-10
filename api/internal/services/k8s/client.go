package k8s

import (
	"context"
	"fmt"
	"io"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ResourceOverheadFactor is the multiplier applied to resource requests
// to reserve capacity for system overhead (kubelet, containerd, OS)
const ResourceOverheadFactor = 0.90 // 10% reserved for system

// StaticPortConfig defines a port with a pre-allocated host port
type StaticPortConfig struct {
	Name          string
	ContainerPort int32
	HostPort      int32 // Pre-allocated host port
	Protocol      corev1.Protocol
}

// VolumeConfig defines a volume mount
type VolumeConfig struct {
	Name      string
	MountPath string
	SubPath   string
}

// Client wraps Kubernetes client
type Client struct {
	clientset *kubernetes.Clientset // Standard K8s resources (Pods, PVCs, Nodes, Deployments)
	config    *rest.Config
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

	return &Client{
		clientset: clientset,
		config:    config,
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

// GetPodByLabel finds a pod by label selector, returns the first running pod found
func (c *Client) GetPodByLabel(ctx context.Context, namespace, labelSelector string) (*corev1.Pod, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Find a running pod
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return &pod, nil
		}
	}

	// If no running pod, return the first one (might be starting)
	if len(pods.Items) > 0 {
		return &pods.Items[0], nil
	}

	return nil, fmt.Errorf("no pods found with label: %s", labelSelector)
}

// StreamPodLogs returns a streaming io.ReadCloser for real-time log following.
// The stream includes the last `tailLines` of historical logs followed by new logs.
// The caller is responsible for closing the returned stream.
func (c *Client) StreamPodLogs(ctx context.Context, namespace, podName, containerName string, tailLines int64) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Container: containerName,
		Follow:    true,
		TailLines: &tailLines,
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to stream pod logs: %w", err)
	}

	return stream, nil
}

// DeploymentParams holds parameters for creating a game server Deployment
type DeploymentParams struct {
	Namespace   string
	Name        string
	Image       string
	NodeName    string
	Ports       []StaticPortConfig
	Volumes     []VolumeConfig
	Env         map[string]string
	CPURequest  string
	MemRequest  string
	PVCName     string
	Labels      map[string]string
	GracePeriod int32
}

// CreateGameDeployment creates a Kubernetes Deployment for a game server with supervisor
func (c *Client) CreateGameDeployment(ctx context.Context, params DeploymentParams) error {
	// Build environment variables
	var envVars []corev1.EnvVar
	for key, value := range params.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	// Build container ports with hostPort
	var containerPorts []corev1.ContainerPort
	for _, port := range params.Ports {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          port.Name,
			ContainerPort: port.ContainerPort,
			HostPort:      port.HostPort,
			Protocol:      port.Protocol,
		})
	}

	// Build volume mounts
	var volumeMounts []corev1.VolumeMount
	for _, vol := range params.Volumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "server-data",
			MountPath: vol.MountPath,
			SubPath:   vol.SubPath,
		})
	}

	// Single PVC volume
	podVolumes := []corev1.Volume{
		{
			Name: "server-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: params.PVCName,
				},
			},
		},
	}

	// Apply overhead factor to resource requests
	cpuQty := resource.MustParse(params.CPURequest)
	memQty := resource.MustParse(params.MemRequest)
	adjustedCPU := resource.NewMilliQuantity(int64(float64(cpuQty.MilliValue())*ResourceOverheadFactor), resource.DecimalSI)
	adjustedMemory := resource.NewQuantity(int64(float64(memQty.Value())*ResourceOverheadFactor), resource.BinarySI)

	replicas := int32(1)
	gracePeriod := int64(params.GracePeriod)
	if gracePeriod == 0 {
		gracePeriod = 30
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: params.Namespace,
			Labels:    params.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: params.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: params.Labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            "gshub-supervisor",
					TerminationGracePeriodSeconds: &gracePeriod,
					DNSConfig: &corev1.PodDNSConfig{
						Options: []corev1.PodDNSConfigOption{
							{
								Name:  "ndots",
								Value: func() *string { s := "2"; return &s }(),
							},
						},
					},
					// Hard node affinity: Pin to the specific node where port is allocated
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/hostname",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{params.NodeName},
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:         "supervisor",
							Image:        params.Image,
							Env:          envVars,
							Ports:        containerPorts,
							VolumeMounts: volumeMounts,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *adjustedCPU,
									corev1.ResourceMemory: *adjustedMemory,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       15,
								FailureThreshold:    2,
							},
						},
					},
					Volumes: podVolumes,
				},
			},
		},
	}

	_, err := c.clientset.AppsV1().Deployments(params.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}

	return nil
}

// GetGameDeployment retrieves a game server Deployment
func (c *Client) GetGameDeployment(ctx context.Context, namespace, name string) (*appsv1.Deployment, error) {
	deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Deployment: %w", err)
	}
	return deployment, nil
}

// DeleteGameDeployment deletes a game server Deployment
func (c *Client) DeleteGameDeployment(ctx context.Context, namespace, name string) error {
	err := c.clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Deployment: %w", err)
	}
	return nil
}

// ScaleGameDeployment scales a Deployment to the specified number of replicas
func (c *Client) ScaleGameDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	scale, err := c.clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Deployment scale: %w", err)
	}

	scale.Spec.Replicas = replicas
	_, err = c.clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale Deployment: %w", err)
	}

	return nil
}

// DeploymentExists checks if a Deployment exists
func (c *Client) DeploymentExists(ctx context.Context, namespace, name string) (bool, error) {
	_, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check Deployment: %w", err)
	}
	return true, nil
}
