package site

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	netv1a3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	extv1b1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/acquia/fn-drupal-operator/pkg/common"
	"github.com/acquia/fn-drupal-operator/pkg/testhelpers"
	goldenHelper "github.com/acquia/fn-go-utils/pkg/testhelpers"
)

const maxReconcileIters = 50

func successfulReconcile(t *testing.T, req reconcile.Request, objs []runtime.Object) *ReconcileSite {
	r := BuildFakeReconcile(objs)

	var err error
	var res = reconcile.Result{Requeue: true}
	// reconcile until we're finished with the loop
	for i := 0; res.Requeue || res.RequeueAfter > 0; i++ {
		if i > maxReconcileIters {
			t.Fatal("Maximum Reconcile() iterations reached")
		}

		res, err = r.Reconcile(req)
		require.NoError(t, err) // accept no errors in this test
	}

	return r
}

func Test_ReconcileIstio(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	istioWasEnabled := common.IsIstioEnabled()

	// objects to track in the fake client
	objects := []runtime.Object{
		drupalApplication,
		drupalEnvironment,
		testSite,
		testDatabase,
		testDBUserSecret,
		dbAdminSecret,
	}

	t.Run("nonIstio", func(t *testing.T) {
		common.SetIsIstioEnabled_ForTestsOnly(false)
		siteKey := types.NamespacedName{
			Name:      testSite.Name,
			Namespace: testSite.Namespace,
		}
		request := reconcile.Request{NamespacedName: siteKey}
		r := successfulReconcile(t, request, objects)

		ing := &extv1b1.Ingress{}
		err := r.client.Get(
			context.TODO(),
			siteKey,
			ing,
		)
		require.NoError(t, err)

		require.True(t, goldenHelper.GoldenSpec(t, "nonIstioIngress", ing))
	})

	t.Run("istio", func(t *testing.T) {
		common.SetIsIstioEnabled_ForTestsOnly(true)

		siteKey := types.NamespacedName{
			Name:      testSite.Name,
			Namespace: testSite.Namespace,
		}
		request := reconcile.Request{NamespacedName: siteKey}
		r := successfulReconcile(t, request, objects)

		ing := &extv1b1.Ingress{}
		err := r.client.Get(context.TODO(), siteKey, ing)
		require.Error(t, err, errors.IsNotFound)

		virtualService := &netv1a3.VirtualService{}
		err = r.client.Get(context.TODO(), siteKey, virtualService)
		require.NoError(t, err)

		require.True(t, goldenHelper.GoldenSpec(t, "istioVirtualService", virtualService))
	})
	common.SetIsIstioEnabled_ForTestsOnly(istioWasEnabled)
}

// BuildFakeReconcile return reconcile with fake client, schemes and runtime objects
func BuildFakeReconcile(objects []runtime.Object) *ReconcileSite {
	c := testhelpers.NewFakeClient(objects)

	// create a ReconcileDrupalApplication object with the scheme and fake client
	return &ReconcileSite{
		client: c,
		scheme: scheme.Scheme,
	}
}
