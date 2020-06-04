package v1alpha1

import (
	v1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "operator-sdk generate k8s && operator-sdk generate crds" to regenerate code after modifying this file
// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

// CommandSpec defines the desired state of Command
// +k8s:openapi-gen=true
type CommandSpec struct {
	// TargetRef specifies the resource to base this Command's Job off of (replicating Pod spec, etc.)
	TargetRef TargetRef `json:"targetRef"`
	// Command specified the shell command to run in the Command's Job, as an array of args.
	// +listType=set
	Command       []string             `json:"command"`
	Retries       int32                `json:"retries,omitempty"`       // +optional
	RunAsRoot     bool                 `json:"runAsRoot,omitempty"`     // +optional
	RestartPolicy corev1.RestartPolicy `json:"restartPolicy,omitempty"` // +optional

	Schedule          string                         `json:"schedule,omitempty"`          // +optional
	Suspend           bool                           `json:"suspend,omitempty"`           // +optional
	ConcurrencyPolicy batchv1beta1.ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"` // +optional

	// Resources specifies the resource requests and limits for the Command Job's main container, overriding the values
	// from the TargetRef's Spec.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"` // +optional

	ActiveDeadlineSeconds         *int64 `json:"activeDeadlineSeconds,omitempty"`         // +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"` // +optional

	// Image optionally specifies an OCI image repo:tag to use in the Command Job's container, overriding the image used
	// by the TargetRef resource.
	Image string `json:"image,omitempty"` // +optional

	// InitContainers defines one or more init containers that will be created before the main container. The spec
	// assigned to this field is used verbatim; no additional volumes, labels, or, env vars are added. These must be
	// defined explicitly on the spec if they are needed.
	// +listType=set
	InitContainers []corev1.Container `json:"initContainers,omitempty"` // +optional

	// AdditionalLabels specifies labels to set or override on the main container
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"` // +optional

	// AdditionalEnvVars specifies additional env vars to set on the main container
	// +listType=set
	AdditionalEnvVars []corev1.EnvVar `json:"additionalEnvVars,omitempty"` // +optional

	// AdditionalVolumes specifies additional volumes to define on the Pod(s) the Command creates
	// +listType=set
	AdditionalVolumes []corev1.Volume `json:"additionalVolumes,omitempty"` // +optional

	// AdditionalVolumeMounts specifies additional volume mounts on the main container
	// +listType=set
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"` // +optional
}

type TargetRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
}

// CommandStatus defines the observed state of Command
// +k8s:openapi-gen=true
type CommandStatus struct {
	Job     v1.JobStatus               `json:"job,omitempty"`     // +optional
	CronJob batchv1beta1.CronJobStatus `json:"cronJob,omitempty"` // +optional
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Command is the Schema for the commands API
// +kubebuilder:resource:shortName=cmd;cmds,scope=Namespaced
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Command",type="string",JSONPath=".spec.command"
// +kubebuilder:printcolumn:name="Target Version",type="string",JSONPath=".spec.targetRef.apiVersion",priority=1
// +kubebuilder:printcolumn:name="Target Kind",type="string",JSONPath=".spec.targetRef.kind",priority=1
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=".spec.targetRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Command struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CommandSpec   `json:"spec,omitempty"`
	Status CommandStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CommandList contains a list of Command
type CommandList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Command `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Command{}, &CommandList{})
}
