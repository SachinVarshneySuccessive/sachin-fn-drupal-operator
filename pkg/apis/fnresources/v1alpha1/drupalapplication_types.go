package v1alpha1

// IMPORTANT: Run "operator-sdk generate k8s && operator-sdk generate crds"
// to regenerate code after modifying this file.
// SEE: https://book.kubebuilder.io/reference/generating-crd.html

import (
	"fmt"

	"github.com/acquia/fn-go-utils/pkg/operatorutils"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type ApplicationId string

var appChildLabels = []string{
	ApplicationIdLabel,
}

// DrupalApplicationSpec defines the desired state of a Drupal Application
// +k8s:openapi-gen=true
type DrupalApplicationSpec struct {
	ImageRepo string `json:"imageRepo,omitempty"` // +optional
	GitRepo   string `json:"gitRepo"`
}

// DrupalEnvironmentRef defines a reference to a DrupalEnvironment
type DrupalEnvironmentRef struct {
	Name          string    `json:"name"`
	Namespace     string    `json:"namespace"`
	EnvironmentID string    `json:"environmentID,omitempty"` // +optional
	UID           types.UID `json:"uid"`
}

// DrupalApplicationStatus defines the observed state of a Drupal Application
// +k8s:openapi-gen=true
type DrupalApplicationStatus struct {
	NumEnvironments int32 `json:"numEnvironments"`
	// +listType=set
	Environments []DrupalEnvironmentRef `json:"environments,omitempty"` // +optional
}

// DrupalApplication is the Schema for the drupalapplications API
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName=drapps;drapp,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Envs",type="integer",JSONPath=".status.numEnvironments"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type DrupalApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DrupalApplicationSpec   `json:"spec,omitempty"`
	Status DrupalApplicationStatus `json:"status,omitempty"` // +optional
}

// DrupalApplicationList contains a list of DrupalApplication
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DrupalApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DrupalApplication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DrupalApplication{}, &DrupalApplicationList{})
}

var _ operatorutils.ResourceWithId = &DrupalApplication{}

func (a *DrupalApplication) NewList() runtime.Object {
	return &DrupalApplicationList{}
}

func (a *DrupalApplication) IdLabel() string {
	return ApplicationIdLabel
}

func (a DrupalApplication) Id() ApplicationId {
	return ApplicationId(a.GetLabels()[a.IdLabel()])
}

func (a *DrupalApplication) SetId(value string) {
	if a.GetLabels() == nil {
		a.SetLabels(map[string]string{})
	}
	a.ObjectMeta.Labels[a.IdLabel()] = value
}

func (a DrupalApplication) ChildLabels() map[string]string {
	appLabels := a.GetLabels()
	if appLabels == nil {
		return nil
	}

	ls := make(map[string]string, len(appChildLabels))
	for _, val := range appChildLabels {
		ls[val] = appLabels[val]
	}

	return ls
}

var _ webhook.Validator = &DrupalApplication{}

func (a *DrupalApplication) ValidateCreate() error {
	log := logf.Log.WithName("drupalapplicationvalidator").WithValues("operation", "create")
	return validateApp(log, a, &DrupalApplication{})
}

func (a *DrupalApplication) ValidateUpdate(old runtime.Object) error {
	log := logf.Log.WithName("drupalapplicationvalidator").WithValues("operation", "update")
	olda, ok := old.(*DrupalApplication)
	if !ok {
		return fmt.Errorf("Invalid old object passed.")
	}
	return validateApp(log, a, olda)
}

func (a *DrupalApplication) ValidateDelete() error {
	return nil
}

func validateApp(log logr.Logger, a *DrupalApplication, olda *DrupalApplication) error {
	// THIS IS JUST AN EXAMPLE
	// The behavior is essentially anytime you annotate an application
	// with `deny: "true"`, this webhook will deny the request UNLESS
	// it was previously annotated with "deny: allow-next"
	// Once you've set it to "deny : allow-next", you can set `deny: "true"`
	// on the server.  At which point, ALL updates will be refused until you
	// set "allow-next" again which allows you to remove the annotation.
	// Delete is exempt from this behavior.
	ann := a.GetAnnotations()
	oldann := olda.GetAnnotations()

	if ann["deny"] == "true" && oldann["deny"] != "allow-next" {
		log.Info("Denying your request.")
		return fmt.Errorf("request denied.  change 'deny' to 'allow-next' to allow.")
	}

	if oldann["deny"] == "true" && ann["deny"] != "allow-next" {
		log.Info("Denying your request.")
		return fmt.Errorf("request denied.  change 'deny' to 'allow-next' to allow.")
	}

	return nil
}
