package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// NodeadmConfigSpec defines the desired state of NodeadmConfig.
type NodeadmConfigSpec struct {
	// Kubelet contains options for kubelet.
	// +optional
	Kubelet *KubeletOptions `json:"kubelet,omitempty"`

	// Containerd contains options for containerd.
	// +optional
	Containerd *ContainerdOptions `json:"containerd,omitempty"`

	// FeatureGates holds key-value pairs to enable or disable application features.
	// +optional
	FeatureGates map[Feature]bool `json:"featureGates,omitempty"`

	// PreNodeadmCommands specifies extra commands to run before bootstrapping nodes.
	// +optional
	PreNodeadmCommands []string `json:"preNodeadmCommands,omitempty"`

	// Files specifies extra files to be passed to user_data upon creation.
	// +optional
	Files []File `json:"files,omitempty"`

	// Users specifies extra users to add.
	// +optional
	Users []User `json:"users,omitempty"`

	// NTP specifies NTP configuration.
	// +optional
	NTP *NTP `json:"ntp,omitempty"`

	// DiskSetup specifies options for the creation of partition tables and file systems on devices.
	// +optional
	DiskSetup *DiskSetup `json:"diskSetup,omitempty"`

	// Mounts specifies a list of mount points to be setup.
	// +optional
	Mounts []MountPoints `json:"mounts,omitempty"`

	// Hybrid contains configuration for EKS Hybrid Nodes.
	// When specified, the NodeadmConfig generates userdata for hybrid node
	// bootstrapping instead of standard EC2 node bootstrapping.
	// Hybrid mode is mutually exclusive with EC2-specific options.
	// +optional
	Hybrid *HybridOptions `json:"hybrid,omitempty"`
}

// HybridOptions defines configuration for EKS Hybrid Nodes.
// When specified, the NodeadmConfig generates userdata for hybrid node
// bootstrapping instead of standard EC2 node bootstrapping.
type HybridOptions struct {
	// SSM configures SSM-based authentication for hybrid nodes.
	// This is required for hybrid node support.
	// +kubebuilder:validation:Required
	SSM *HybridSSMOptions `json:"ssm"`
}

// HybridSSMOptions configures SSM activation-based authentication for hybrid nodes.
// Either ActivationRef or ActivationConfig must be specified, but not both.
type HybridSSMOptions struct {
	// ActivationRef references an existing Secret containing SSM activation credentials.
	// The Secret must contain 'activationId' and 'activationCode' keys.
	// When specified, the controller will not create or manage SSM activations.
	// +optional
	ActivationRef *SSMActivationReference `json:"activationRef,omitempty"`

	// ActivationConfig specifies parameters for automatically creating an SSM activation.
	// The controller will create the activation and store credentials in a Secret.
	// The activation will be deleted when the NodeadmConfig is deleted.
	// +optional
	ActivationConfig *SSMActivationConfig `json:"activationConfig,omitempty"`
}

// SSMActivationReference references a Secret containing pre-created SSM activation credentials.
type SSMActivationReference struct {
	// Name is the name of the Secret in the same namespace as the NodeadmConfig.
	// The Secret must contain 'activationId' and 'activationCode' keys.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// SSMActivationConfig specifies parameters for creating an SSM hybrid activation.
type SSMActivationConfig struct {
	// IAMRoleARN is the ARN of the IAM role that hybrid nodes will assume.
	// This role must have the necessary permissions for EKS hybrid nodes
	// and trust policy allowing ssm.amazonaws.com to assume it.
	// See: https://docs.aws.amazon.com/eks/latest/userguide/hybrid-nodes-creds.html
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^arn:aws(-[a-z]+)?:iam::[0-9]{12}:role/.+$`
	IAMRoleARN string `json:"iamRoleARN"`

	// RegistrationLimit is the maximum number of hybrid nodes that can register
	// using this activation. Each NodeadmConfig creates its own activation,
	// so this typically corresponds to the number of nodes using this config.
	// Minimum: 1, Maximum: 1000, Default: 1
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	// +optional
	RegistrationLimit *int32 `json:"registrationLimit,omitempty"`

	// ExpirationDays is the number of days until the activation expires.
	// After expiration, no new nodes can register using this activation,
	// but already-registered nodes are unaffected.
	// Minimum: 1, Maximum: 30, Default: 7
	// +kubebuilder:default=7
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=30
	// +optional
	ExpirationDays *int32 `json:"expirationDays,omitempty"`

	// Tags are key-value pairs to apply to the SSM activation.
	// These tags help identify and manage activations in the AWS console.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
}

// KubeletOptions are additional parameters passed to kubelet.
type KubeletOptions struct {
	// Config is a KubeletConfiguration that will be merged with the defaults.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Config *runtime.RawExtension `json:"config,omitempty"`

	// Flags are command-line kubelet arguments that will be appended to the defaults.
	// +optional
	Flags []string `json:"flags,omitempty"`
}

// ContainerdOptions are additional parameters passed to containerd.
type ContainerdOptions struct {
	// Config is an inline containerd configuration TOML that will be merged with the defaults.
	// +optional
	Config string `json:"config,omitempty"`

	// BaseRuntimeSpec is the OCI runtime specification upon which all containers will be based.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	BaseRuntimeSpec *runtime.RawExtension `json:"baseRuntimeSpec,omitempty"`
}

// Feature specifies which feature gate should be toggled.
// +kubebuilder:validation:Enum=InstanceIdNodeName;FastImagePull
type Feature string

const (
	// FeatureInstanceIDNodeName  will use EC2 instance ID as node name.
	FeatureInstanceIDNodeName Feature = "InstanceIdNodeName"
	// FeatureFastImagePull enables a parallel image pull for container images.
	FeatureFastImagePull Feature = "FastImagePull"
)

// GetConditions returns the observations of the operational state of the NodeadmConfig resource.
func (r *NodeadmConfig) GetConditions() clusterv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the NodeadmConfig to the predescribed clusterv1.Conditions.
func (r *NodeadmConfig) SetConditions(conditions clusterv1beta1.Conditions) {
	r.Status.Conditions = conditions
}

// NodeadmConfigStatus defines the observed state of NodeadmConfig.
type NodeadmConfigStatus struct {
	// Ready indicates the BootstrapData secret is ready to be consumed.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// DataSecretName is the name of the secret that stores the bootstrap data script.
	// +optional
	DataSecretName *string `json:"dataSecretName,omitempty"`

	// FailureReason will be set on non-retryable errors.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// FailureMessage will be set on non-retryable errors.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions defines current service state of the NodeadmConfig.
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	// SSMActivation contains status information about the SSM activation
	// when hybrid mode is enabled with auto-created activations.
	// +optional
	SSMActivation *SSMActivationStatus `json:"ssmActivation,omitempty"`
}

// SSMActivationStatus contains status information for an auto-created SSM activation.
type SSMActivationStatus struct {
	// ActivationID is the ID of the SSM activation.
	// +optional
	ActivationID *string `json:"activationID,omitempty"`

	// SecretName is the name of the Secret containing activation credentials.
	// +optional
	SecretName *string `json:"secretName,omitempty"`

	// ExpirationTime is when the activation expires.
	// +optional
	ExpirationTime *metav1.Time `json:"expirationTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NodeadmConfig is the Schema for the nodeadmconfigs API.
type NodeadmConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeadmConfigSpec   `json:"spec,omitempty"`
	Status NodeadmConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NodeadmConfigList contains a list of NodeadmConfig.
type NodeadmConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeadmConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeadmConfig{}, &NodeadmConfigList{})
}
