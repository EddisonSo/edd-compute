package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	clientset *kubernetes.Clientset
}

func NewClient() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	return &Client{clientset: clientset}, nil
}

// CreateNamespace creates a namespace for a container
func (c *Client) CreateNamespace(ctx context.Context, name string, userID int64, containerID string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"edd-compute":  "true",
				"user-id":      fmt.Sprintf("%d", userID),
				"container-id": containerID,
			},
		},
	}

	_, err := c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create namespace: %w", err)
	}
	return nil
}

// DeleteNamespace deletes a container namespace and all resources in it
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	err := c.clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("delete namespace: %w", err)
	}
	return nil
}

// CreateSSHSecret creates a secret with SSH authorized_keys
func (c *Client) CreateSSHSecret(ctx context.Context, namespace string, authorizedKeys string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ssh-keys",
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"authorized_keys": authorizedKeys,
		},
	}

	_, err := c.clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create ssh secret: %w", err)
	}
	return nil
}

// CreatePVC creates a persistent volume claim for container storage
func (c *Client) CreatePVC(ctx context.Context, namespace string, storageGB int) error {
	storageClassName := "local-path"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage",
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: parseQuantity(fmt.Sprintf("%dGi", storageGB)),
				},
			},
		},
	}

	_, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create pvc: %w", err)
	}
	return nil
}

// CreateNetworkPolicy creates network isolation policy
func (c *Client) CreateNetworkPolicy(ctx context.Context, namespace string) error {
	udpProtocol := corev1.ProtocolUDP
	dnsPort := int32(53)

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "isolation",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{}, // Allow all ingress
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					// Allow DNS
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &udpProtocol,
							Port:     &intOrString{IntVal: dnsPort},
						},
					},
				},
				{
					// Allow internet, block internal
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR:   "0.0.0.0/0",
								Except: []string{"10.0.0.0/8"},
							},
						},
					},
				},
			},
		},
	}

	_, err := c.clientset.NetworkingV1().NetworkPolicies(namespace).Create(ctx, policy, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create network policy: %w", err)
	}
	return nil
}

// CreatePod creates the container pod
func (c *Client) CreatePod(ctx context.Context, namespace string, image string, memoryMB int) error {
	defaultMode := int32(0600)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "container",
			Namespace: namespace,
			Labels: map[string]string{
				"app": "compute-container",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: image,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: parseQuantity(fmt.Sprintf("%dMi", memoryMB)),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: parseQuantity(fmt.Sprintf("%dMi", memoryMB)),
						},
					},
					Ports: []corev1.ContainerPort{
						{ContainerPort: 22, Name: "ssh"},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "storage",
							MountPath: "/home/dev",
						},
						{
							Name:      "ssh-keys",
							MountPath: "/root/.ssh",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "storage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "storage",
						},
					},
				},
				{
					Name: "ssh-keys",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  "ssh-keys",
							DefaultMode: &defaultMode,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}

	_, err := c.clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create pod: %w", err)
	}
	return nil
}

// CreateLoadBalancer creates a LoadBalancer service for the container
func (c *Client) CreateLoadBalancer(ctx context.Context, namespace string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lb",
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				"app": "compute-container",
			},
			Ports: []corev1.ServicePort{
				{Name: "ssh", Port: 22, TargetPort: intOrString{IntVal: 22}},
				{Name: "http", Port: 80, TargetPort: intOrString{IntVal: 80}},
				{Name: "https", Port: 443, TargetPort: intOrString{IntVal: 443}},
				{Name: "dev-3000", Port: 3000, TargetPort: intOrString{IntVal: 3000}},
				{Name: "dev-8080", Port: 8080, TargetPort: intOrString{IntVal: 8080}},
			},
		},
	}

	_, err := c.clientset.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create load balancer: %w", err)
	}
	return nil
}

// GetServiceExternalIP gets the external IP of a LoadBalancer service
func (c *Client) GetServiceExternalIP(ctx context.Context, namespace string) (string, error) {
	svc, err := c.clientset.CoreV1().Services(namespace).Get(ctx, "lb", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get service: %w", err)
	}

	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		if svc.Status.LoadBalancer.Ingress[0].IP != "" {
			return svc.Status.LoadBalancer.Ingress[0].IP, nil
		}
	}
	return "", nil // No IP assigned yet
}

// GetPodStatus gets the status of a pod
func (c *Client) GetPodStatus(ctx context.Context, namespace string) (string, error) {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, "container", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "not_found", nil
		}
		return "", fmt.Errorf("get pod: %w", err)
	}

	switch pod.Status.Phase {
	case corev1.PodPending:
		return "pending", nil
	case corev1.PodRunning:
		return "running", nil
	case corev1.PodSucceeded:
		return "stopped", nil
	case corev1.PodFailed:
		return "failed", nil
	default:
		return "unknown", nil
	}
}

// DeletePod deletes the container pod
func (c *Client) DeletePod(ctx context.Context, namespace string) error {
	err := c.clientset.CoreV1().Pods(namespace).Delete(ctx, "container", metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("delete pod: %w", err)
	}
	return nil
}
