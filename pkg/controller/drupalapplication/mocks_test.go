package drupalapplication

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/testhelpers"
)

const (
	testNamespace = "wlgore-app"
	testAppName   = "wlgore-app"
	testImageRepo = "881217801864.dkr.ecr.us-east-1.amazonaws.com/kpoc/default"
	testGitRepo   = "nebula@svn-2.archteam.srvs.ahdev.co:nebula.git"
)

var testAppID = "d8de5846-fbec-4a35-b888-aed09bb1733b"
var testEnvProdID = "e5462ba5-09d0-49fa-a5b3-8c82d85f4c9e"
var testEnvStgID = "a9ccc445-8e8f-4885-b39e-9e16edd34da6"

var (
	drupalApplicationWithoutID = &fnv1alpha1.DrupalApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: testAppName,
		},
		Spec: fnv1alpha1.DrupalApplicationSpec{
			ImageRepo: testImageRepo,
			GitRepo:   testGitRepo,
		},
	}

	drupalEnvironmentWithoutLabel = &fnv1alpha1.DrupalEnvironment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAppName + "-prod",
			Namespace: testNamespace + "-prod",
		},
		Spec: fnv1alpha1.DrupalEnvironmentSpec{},
	}

	drupalApplicationWithoutImageRepo = &fnv1alpha1.DrupalApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: testAppName,
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
			},
		},
		Spec: fnv1alpha1.DrupalApplicationSpec{
			GitRepo: testGitRepo,
		},
	}

	drupalEnvironmentProdWithLabel = &fnv1alpha1.DrupalEnvironment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAppName + "-prod",
			Namespace: testNamespace + "-prod",
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
				fnv1alpha1.EnvironmentIdLabel: testEnvProdID,
			},
		},
		Spec: fnv1alpha1.DrupalEnvironmentSpec{},
	}
	drupalEnvironmentStgWithLabel = &fnv1alpha1.DrupalEnvironment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAppName + "-stg",
			Namespace: testNamespace + "-stg",
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
				fnv1alpha1.EnvironmentIdLabel: testEnvStgID,
			},
		},
		Spec: fnv1alpha1.DrupalEnvironmentSpec{},
	}
)

// buildFakeReconcile return reconcile with fake client, schemes and runtime objects
func buildFakeReconcile(objects []runtime.Object) *ReconcileDrupalApplication {
	client := testhelpers.NewFakeClient(objects)

	// create a ReconcileDrupalApplication object with the scheme and fake client
	return &ReconcileDrupalApplication{
		client: client,
		scheme: scheme.Scheme,
	}
}
