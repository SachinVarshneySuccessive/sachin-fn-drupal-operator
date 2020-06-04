package v1alpha1

// IMPORTANT: Run "operator-sdk generate k8s && operator-sdk generate crds"
// to regenerate code after modifying this file.
// SEE: https://book.kubebuilder.io/reference/generating-crd.html

import (
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	"github.com/acquia/fn-go-utils/pkg/operatorutils"
)

type EnvironmentId string

// Describes the status of the environment.
type DrupalEnvironmentStatusType string

const (
	DrupalEnvironmentStatusSyncing     DrupalEnvironmentStatusType = "Syncing"
	DrupalEnvironmentStatusDeploying   DrupalEnvironmentStatusType = "Deploying"
	DrupalEnvironmentStatusSynced      DrupalEnvironmentStatusType = "Synced"
	DrupalEnvironmentStatusUnstable    DrupalEnvironmentStatusType = "Unstable"
	DrupalEnvironmentStatusDeployError DrupalEnvironmentStatusType = "DeployError"
	DrupalEnvironmentStatusDeleting    DrupalEnvironmentStatusType = "Deleting"
)

var envChildLabels = []string{
	ApplicationIdLabel,
	EnvironmentIdLabel,
}

// DrupalEnvironmentSpec defines the desired state of DrupalEnvironment
// +k8s:openapi-gen=true
type DrupalEnvironmentSpec struct {
	Application                string      `json:"application"`
	Production                 bool        `json:"production"`
	EFSID                      string      `json:"efsid"`
	GitRef                     string      `json:"gitRef"`
	Stage                      string      `json:"stage"`
	CustomEnvironmentVariables []v1.EnvVar `json:"customEnvironmentVariables,omitempty"` // +optional

	Drupal SpecDrupal `json:"drupal"`
	Apache SpecApache `json:"apache"`
	Phpfpm SpecPhpFpm `json:"phpfpm"`
}

// SpecDrupal represents drupalenvironment.spec.drupal
type SpecDrupal struct {
	Tag                            string        `json:"tag"`
	PullPolicy                     v1.PullPolicy `json:"pullPolicy"`
	MinReplicas                    int32         `json:"minReplicas"`
	MaxReplicas                    int32         `json:"maxReplicas"`
	TargetCPUUtilizationPercentage *int32        `json:"targetCPUUtilizationPercentage,omitempty"`

	Liveness  HTTPProbe `json:"livenessProbe"`
	Readiness HTTPProbe `json:"readinessProbe"`
}

// SpecApache represents drupalenvironment.spec.apache
type SpecApache struct {
	CustomImage string `json:"customImage,omitempty"` // +optional
	Tag         string `json:"tag"`

	WebRoot string    `json:"webRoot"`
	Cpu     Resources `json:"cpu"`
	Memory  Resources `json:"memory"`
}

// SpecPhpFpm represents drupalenvironment.spec.phpfpm
type SpecPhpFpm struct {
	CustomImage string `json:"customImage,omitempty"` // +optional
	Tag         string `json:"tag"`

	Procs                           int32     `json:"procs"`
	MaxInputVars                    int32     `json:"maxInputVars"`
	MaxExecutionTime                int32     `json:"maxExecutionTime"`
	ProcMemoryLimitMiB              int32     `json:"procMemoryLimitMiB"`
	PostMaxSizeMiB                  int32     `json:"postMaxSizeMiB"`
	OpcacheMemoryLimitMiB           int32     `json:"opcacheMemoryLimitMiB"`
	OpcacheInternedStringsBufferMiB int32     `json:"opcacheInternedStringsBufferMiB"`
	ApcMemoryLimitMiB               int32     `json:"apcMemoryLimitMiB"`
	Cpu                             Resources `json:"cpu"`

	NewRelicSecret  string `json:"newRelicSecret,omitempty"`  // +optional
	NewRelicAppName string `json:"newRelicAppName,omitempty"` // +optional
}

// Resources specifies container resource requests and limits
type Resources struct {
	Request resource.Quantity `json:"request"`
	Limit   resource.Quantity `json:"limit"`
}

// HTTPProbe specifies a container's HTTP liveness/readiness probe
type HTTPProbe struct {
	Enabled          bool   `json:"enabled"`
	HTTPPath         string `json:"httpPath"`
	TimeoutSeconds   int32  `json:"timeoutSeconds"`
	FailureThreshold int32  `json:"failureThreshold"`
	SuccessThreshold int32  `json:"successThreshold"`
	PeriodSeconds    int32  `json:"periodSeconds"`
}

// DrupalEnvironmentStatus defines the observed state of DrupalEnvironment
// +k8s:openapi-gen=true
type DrupalEnvironmentStatus struct {
	NumDrupal int32                       `json:"numDrupal"`
	Status    DrupalEnvironmentStatusType `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DrupalEnvironment is the Schema for the drupalenvironments API
// +kubebuilder:resource:shortName=drenv;drenvs,scope=Namespaced
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.numDrupal",description="The number of Drupal Pods in ready state"
// +kubebuilder:printcolumn:name="PHP-Tag",type="string",JSONPath=".spec.phpfpm.tag",description="Tagged Version of PHP"
// +kubebuilder:printcolumn:name="Drupal-Tag",type="string",JSONPath=".spec.drupal.tag",description="The tag of Drupal Image"
// +kubebuilder:printcolumn:name="Stage",type="string",JSONPath=".spec.stage",description="The environment's stage name"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status",description="Current status of the environment"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Prod",priority=1,type="boolean",JSONPath=".spec.production",description="Environment is Production"
// +kubebuilder:printcolumn:name="Git-Ref",priority=1,type="string",JSONPath=".spec.gitRef",description="Deployed git ref"
// +kubebuilder:printcolumn:name="Custom-Apache",priority=1,type="string",JSONPath=".spec.apache.customImage",description="Custom apache image"
// +kubebuilder:printcolumn:name="Custom-PHP",priority=1,type="string",JSONPath=".spec.phpfpm.customImage",description="Custom php-fpm image"
type DrupalEnvironment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DrupalEnvironmentSpec   `json:"spec,omitempty"`
	Status DrupalEnvironmentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DrupalEnvironmentList contains a list of DrupalEnvironment
type DrupalEnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DrupalEnvironment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DrupalEnvironment{}, &DrupalEnvironmentList{})
}

var _ operatorutils.ResourceWithId = &DrupalEnvironment{}

func (e *DrupalEnvironment) NewList() runtime.Object {
	return &DrupalEnvironmentList{}
}

func (e *DrupalEnvironment) IdLabel() string {
	return EnvironmentIdLabel
}

func (e DrupalEnvironment) Id() EnvironmentId {
	return EnvironmentId(e.GetLabels()[e.IdLabel()])
}

func (e *DrupalEnvironment) SetId(value string) {
	if e.GetLabels() == nil {
		e.SetLabels(map[string]string{})
	}
	e.ObjectMeta.Labels[e.IdLabel()] = value
}

func (e DrupalEnvironment) ChildLabels() map[string]string {
	envLabels := e.GetLabels()
	if envLabels == nil {
		return nil
	}

	ls := make(map[string]string, len(envChildLabels))
	for _, val := range envChildLabels {
		ls[val] = envLabels[val]
	}

	return ls
}

/*****************
**  Migrations  **
*****************/

// Ensure DrupalEnvironment implements "versionedType"
var _ versionedType = &DrupalEnvironment{}

// SpecVersion returns the latest resource Spec version number for this type. Any resources with an earlier version
// label (or no label) should be processed by doMigrate().
func (e *DrupalEnvironment) SpecVersion() string {
	return "2"
}

func (e *DrupalEnvironment) migrationNeeded() bool {
	return e.Spec.Stage == ""
}

func (e *DrupalEnvironment) doMigrate() {
	// Migrate version "1" -> "2":
	// Back-fill Stage field based on Name (and ensure Production field is in sync)

	e.Spec.Production = false
	if strings.Contains(e.Name, "dev") {
		e.Spec.Stage = "dev"
	} else if strings.Contains(e.Name, "test") {
		e.Spec.Stage = "test"
	} else {
		e.Spec.Production = true
		e.Spec.Stage = "prod"
	}
}
