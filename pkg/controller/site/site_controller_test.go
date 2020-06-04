package site

import (
	"context"
	"testing"
	"time"

	goldenHelper "github.com/acquia/fn-go-utils/pkg/testhelpers"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	extv1b1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"
	"github.com/acquia/fn-drupal-operator/pkg/testhelpers"
)

func TestSiteController_ReconcileDrupalEnvironmentNotFound(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	objects := []runtime.Object{
		siteWithID,
	}

	r := buildFakeReconcile(objects)

	// mock request to simulate Reconcile() being called on an event for a watched resource
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      siteWithID.Name,
			Namespace: siteWithID.Namespace,
		},
	}

	res, err := r.Reconcile(req)
	// expecting finalizer to be added, and must requeue
	// there is separate test for finalizers
	require.Nil(t, err)
	require.Equal(t, res.Requeue, true)

	// Should RequeueAfter if DrupalEnvironment resource doesn't exists
	res, err = r.Reconcile(req)
	require.Nil(t, err)
	require.Equal(t, time.Second*10, res.RequeueAfter)
}

func TestSiteController_ReconcileDrupalApplicationNotFound(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	objects := []runtime.Object{
		siteWithID,
		drupalEnvironment,
	}

	r := buildFakeReconcile(objects)

	// mock request to simulate Reconcile() being called on an event for a watched resource
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      siteWithID.Name,
			Namespace: siteWithID.Namespace,
		},
	}

	res, err := r.Reconcile(req)
	// expecting finalizer to be added, and must requeue
	// there is separate test for finalizers
	require.Nil(t, err)
	require.Equal(t, res.Requeue, true)

	// Should RequeueAfter if DrupalApplication object doesn't exists
	res, err = r.Reconcile(req)
	require.Nil(t, err)
	require.Equal(t, time.Second*10, res.RequeueAfter)
}

func TestSiteController_ReconcileDatabaseNotFound(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	objects := []runtime.Object{
		siteWithID,
		drupalEnvironment,
		drupalApplication,
	}

	r := buildFakeReconcile(objects)

	// mock request to simulate Reconcile() being called on an event for a watched resource
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      siteWithID.Name,
			Namespace: siteWithID.Namespace,
		},
	}

	res, err := r.Reconcile(req)

	// Should RequeueAfter if Database resource doesn't exists
	res, err = r.Reconcile(req)
	require.Nil(t, err)
	require.Equal(t, time.Second*10, res.RequeueAfter)
}

func TestSiteController_ReconcileSiteWithoutID(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	objects := []runtime.Object{
		siteWithoutID,
	}

	r := buildFakeReconcile(objects)

	// mock request to simulate Reconcile() being called on an event for a watched resource
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      siteWithoutID.Name,
			Namespace: siteWithoutID.Namespace,
		},
	}

	t.Run("should set DrupalApplication Id", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)

		// Validating that Site should have valid UUID
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, siteWithoutID)
		require.Nil(t, err)
		require.Equal(t, testhelpers.IsValidUUID(string(siteWithoutID.Id())), true)
	})
}

func TestSiteController_Reconcile(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	objects := []runtime.Object{
		siteWithID,
		testSecondSite,
		drupalEnvironment,
		drupalApplication,
		testDatabase,
		test2ndDatabase,
		testDBUserSecret,
		test2ndDBUserSecret,
		dbAdminSecret,
	}

	r := buildFakeReconcile(objects)

	// mock request to simulate Reconcile() being called on an event for a watched resource
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      siteWithID.Name,
			Namespace: siteWithID.Namespace,
		},
	}

	t.Run("should set site CleanupFinalizer", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)

		// Verifying Finalizers should be set correctly
		site := &fnv1alpha1.Site{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, site)
		require.Nil(t, err)
		require.Equal(t, common.HasFinalizer(site, siteCleanupFinalizer), true)
	})

	t.Run("should set Parent Environment and Parent Application and LinkToEnvironment", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)

		// Verifying the labels of owners should be set correctly
		site := &fnv1alpha1.Site{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, site)
		require.Nil(t, err)

		labels := site.GetLabels()
		require.Equal(t, labels[fnv1alpha1.EnvironmentIdLabel], testEnvID)
		require.Equal(t, labels[fnv1alpha1.ApplicationIdLabel], testAppID)
	})

	t.Run("should reconcile env-config Secret", func(t *testing.T) {
		// Verifying the env-config Secret does not yet exist
		secret := &corev1.Secret{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: "env-config", Namespace: testNamespace}, secret)
		require.True(t, errors.IsNotFound(err))

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verifying the env-config Secret has been created
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "env-config", Namespace: testNamespace}, secret)
		require.NoError(t, err)
		require.True(t, goldenHelper.Golden(t, "first_site_settings.inc", secret.Data[testSiteName+".settings.inc"]))
		require.True(t, goldenHelper.GoldenSpec(t, "Secret", secret))
	})

	t.Run("should reconcile ingress", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)

		// Verify Ingress has been setup correctly
		ingress := &extv1b1.Ingress{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: siteWithID.Name}, ingress)
		require.NoError(t, err)
		require.Equal(t, ingress.Spec.Rules[1].Host, testDomain2)
		require.Nil(t, ingress.Spec.TLS)
	})

	t.Run("should finish reconcile loop", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, false)
	})

	t.Run("should add new Domain and TLS to ingress", func(t *testing.T) {
		site := &fnv1alpha1.Site{}
		_ = r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, site)

		// Add a third domain to Site resource
		site.Spec.Domains = append(site.Spec.Domains, testDomain3)
		site.Spec.Tls = true
		err := r.client.Update(context.TODO(), site)
		require.Nil(t, err)

		// Expecting to update the Site Settings Secret
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)

		// Expecting ingress to update the third site
		res, err = r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)

		// Expecting to finish reconcile
		res, err = r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, false)

		ingress := &extv1b1.Ingress{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: siteWithID.Name}, ingress)
		require.Nil(t, err)

		// checking new site has been added
		require.Equal(t, ingress.Spec.Rules[2].Host, testDomain3)

		// verifying TLS has been setup
		require.Equal(t, ingress.Spec.TLS[0].Hosts[2], testDomain3)

		// verifying default annotations
		annotations := ingress.GetAnnotations()
		require.Equal(t, annotations["certmanager.k8s.io/cluster-issuer"], "letsencrypt-staging")
		require.Equal(t, annotations["kubernetes.io/ingress.class"], "nginx")
	})

	t.Run("should add cert issuer and ingressClass", func(t *testing.T) {
		site := &fnv1alpha1.Site{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, site)
		require.Nil(t, err)

		// Updating the Cert and Ingress
		site.Spec.CertIssuer = "AWS"
		site.Spec.IngressClass = "custom"
		err = r.client.Update(context.TODO(), site)
		require.Nil(t, err)

		// Expecting ingress to update the cert issuer and ingressClass
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)
		// Expecting to finish reconcile
		res, err = r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, false)

		// Fetching the lastest ingress
		ingress := &extv1b1.Ingress{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: siteWithID.Name}, ingress)
		require.Nil(t, err)

		// Validating Cert and Ingress has been updated successfully
		annotations := ingress.GetAnnotations()
		require.Equal(t, annotations["certmanager.k8s.io/cluster-issuer"], "AWS")
		require.Equal(t, annotations["kubernetes.io/ingress.class"], "custom")
	})

	t.Run("fully reconcile second site and verify settings", func(t *testing.T) {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      testSecondSite.Name,
				Namespace: testSecondSite.Namespace,
			},
		}

		// Loop until second site is fully reconciled
		var res reconcile.Result
		var err error
		for i := 0; i < 10; i++ {
			res, err = r.Reconcile(req)
			require.Nil(t, err)

			if !res.Requeue && res.RequeueAfter == 0 {
				break
			}
		}
		require.False(t, res.Requeue)
		require.Zero(t, res.RequeueAfter)

		// Verifying the env-config Secret has been updated
		secret := &corev1.Secret{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "env-config", Namespace: testNamespace}, secret)
		require.NoError(t, err)
		require.True(t, goldenHelper.Golden(t, "second_site_settings.inc", secret.Data[testSecondSiteName+".settings.inc"]))
		require.True(t, goldenHelper.GoldenSpec(t, "Secret", secret))
	})

	t.Run("should delete the site", func(t *testing.T) {
		site := &fnv1alpha1.Site{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: siteWithID.Name}, site)
		require.Nil(t, err)

		// Set DeletionTimestamp on the Site, since the fake client `Delete()` does't behave the same as a real cluster.
		site.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
		err = r.client.Update(context.TODO(), site)
		require.Nil(t, err)

		// Should remove entry from env-config Secret
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.True(t, res.Requeue)

		secret := &corev1.Secret{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "env-config", Namespace: testNamespace}, secret)
		require.NoError(t, err)
		require.Zero(t, string(secret.Data[siteWithID.Name+".settings.inc"]))
		require.True(t, goldenHelper.Golden(t, "second_site_settings.inc", secret.Data[testSecondSiteName+".settings.inc"]))
		require.True(t, goldenHelper.GoldenSpec(t, "Secret", secret))

		// Should remove finalizer
		res, err = r.Reconcile(req)
		require.Nil(t, err)
		require.True(t, res.Requeue)
	})
}
