package testhelpers

import (
	rollouts "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	netclient "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	fnresources "github.com/acquia/fn-drupal-operator/pkg/apis"
	"github.com/acquia/fn-go-utils/pkg/testhelpers"
)

// Compiler complains that the flag is undefined without this somewhere.
var _ = testhelpers.Update

func NewFakeClient(objects []runtime.Object) client.Client {
	if err := fnresources.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	if err := rollouts.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	if err := netclient.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	return fake.NewFakeClientWithScheme(scheme.Scheme, objects...)
}
