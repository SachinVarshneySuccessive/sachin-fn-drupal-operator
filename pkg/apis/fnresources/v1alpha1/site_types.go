package v1alpha1

// IMPORTANT: Run "operator-sdk generate k8s && operator-sdk generate crds"
// to regenerate code after modifying this file.
// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/reference/generating-crd.html

import (
	"strings"

	extv1b1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/acquia/fn-go-utils/pkg/operatorutils"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type DomainMap map[string]string
type SiteId string

var siteChildLabels = []string{
	ApplicationIdLabel,
	EnvironmentIdLabel,
	SiteIdLabel,
}

// SiteSpec defines the desired state of Site
// +k8s:openapi-gen=true
type SiteSpec struct {
	// +listType=set
	Domains      []string    `json:"domains"`
	Environment  string      `json:"environment"`
	Database     string      `json:"database"`
	Install      InstallSpec `json:"install,omitempty"`      // +optional
	Tls          bool        `json:"tls,omitempty"`          // +optional
	IngressClass string      `json:"ingressClass,omitempty"` // +optional
	CertIssuer   string      `json:"certIssuer,omitempty"`   // +optional
}

// Information to install the site
// +k8s:openapi-gen=true
type InstallSpec struct {
	InstallProfile string `json:"installProfile"`
	AdminUsername  string `json:"adminUsername"`
	AdminEmail     string `json:"adminEmail"`
}

// Status describes the status of the site.
type Status string

const (
	SiteSyncingStatus Status = "Syncing"
	SiteSyncedStatus  Status = "Ready"
	SiteCleanUpStatus Status = "Finalizing"
)

type DomainStatus string

const (
	DomainsSyncedStatus   DomainStatus = "Synced"
	DomainsUpdatingStatus DomainStatus = "Updating"
	DomainsCleanUpStatus  DomainStatus = "Finalizing"
)

// SiteStatus defines the observed state of Site
// +k8s:openapi-gen=true
type SiteStatus struct {
	Status  Status       `json:"status"`
	Domains DomainStatus `json:"domains"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Site is the Schema for the sites API
// +kubebuilder:resource:scope=Namespaced
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.status"
// +kubebuilder:printcolumn:name="domains",type="string",JSONPath=".spec.domains"
// +kubebuilder:printcolumn:name="tls",type="string",JSONPath=".spec.tls"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Site struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SiteSpec   `json:"spec,omitempty"`
	Status SiteStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SiteList contains a list of Site
type SiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Site `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Site{}, &SiteList{})
}

var _ operatorutils.ResourceWithId = &Site{}

func (s *Site) NewList() runtime.Object {
	return &SiteList{}
}

func (s *Site) IdLabel() string {
	return SiteIdLabel
}

func (s Site) Id() SiteId {
	return SiteId(s.GetLabels()[s.IdLabel()])
}

func (s *Site) SetId(value string) {
	if s.GetLabels() == nil {
		s.SetLabels(map[string]string{})
	}
	s.ObjectMeta.Labels[s.IdLabel()] = value
}

func (s Site) ChildLabels() map[string]string {
	siteLabels := s.GetLabels()
	if siteLabels == nil {
		return nil
	}

	ls := make(map[string]string, len(siteChildLabels))
	for _, val := range siteChildLabels {
		ls[val] = siteLabels[val]
	}

	return ls
}

func sanitize(old string) string {
	old = strings.ReplaceAll(old, `-`, ``)
	old = strings.ReplaceAll(old, `'`, ``)
	old = strings.ReplaceAll(old, `"`, ``)
	old = strings.ReplaceAll(old, `.`, ``)
	return old
}

func (s *Site) IngressRules() []extv1b1.IngressRule {
	value := extv1b1.IngressRuleValue{
		HTTP: &extv1b1.HTTPIngressRuleValue{
			Paths: []extv1b1.HTTPIngressPath{
				{
					Path: "/",
					Backend: extv1b1.IngressBackend{
						ServiceName: "drupal",
						ServicePort: intstr.FromInt(80),
					},
				},
			},
		},
	}

	rules := make([]extv1b1.IngressRule, len(s.Spec.Domains))
	for i, host := range s.Spec.Domains {
		rules[i] = extv1b1.IngressRule{
			Host:             host,
			IngressRuleValue: value,
		}
	}

	return rules
}

func (s *Site) IngressTLS() []extv1b1.IngressTLS {
	if s.Spec.Tls {
		return []extv1b1.IngressTLS{{
			Hosts:      s.Spec.Domains,
			SecretName: s.Name + "-tls-secret",
		}}
	}
	return nil
}

// Returns site ingress class for kubernetes.io/ingress.class ingress annotation
func (s *Site) IngressClass() string {
	def := "nginx" //Defaults to nginx
	ic := s.Spec.IngressClass
	if ic != "" {
		return ic
	}
	return def
}

// Returns cert-manager issuer for certmanager.k8s.io/cluster-issuer ingress annotation
func (s *Site) IngressCertIssuer() string {
	def := "letsencrypt-staging" //Defaults to letsencrypt-staging
	cmi := s.Spec.CertIssuer
	if cmi != "" {
		return cmi
	}
	return def
}

// SetSiteDomainStatus sets the site domain status.
func (s *Site) SetDomainStatus(status DomainStatus) {
	s.Status.Domains = status
}

// SetStatus sets the site status.
func (s *Site) SetStatus(status Status) {
	s.Status.Status = status
}

// DrupalSiteName returns the site name to be used for Drupal sites.php and site files path.
func DrupalSiteName(site *Site) string {
	return site.Name
}
