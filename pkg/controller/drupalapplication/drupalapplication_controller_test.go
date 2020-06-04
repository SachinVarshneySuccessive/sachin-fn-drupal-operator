package drupalapplication

import (
	"context"
	"crypto/sha1"
	"fmt"
	"testing"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/testhelpers"
	goldenHelper "github.com/acquia/fn-go-utils/pkg/testhelpers"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestApplicationController_UnknownName(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	unknownName := "unknown"

	// objects to track in the fake client
	fakeObjects := []runtime.Object{
		drupalApplicationWithoutID,
	}

	r := buildFakeReconcile(fakeObjects)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: unknownName,
		},
	}

	// should throw error if the DrupalApplication with provided name doesn't exists.
	_, err := r.Reconcile(req)
	require.Nil(t, err)
}

func TestApplicationController_ReconcileWithoutEnvironment(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	fakeObjects := []runtime.Object{
		drupalApplicationWithoutID,
		drupalEnvironmentWithoutLabel,
	}

	r := buildFakeReconcile(fakeObjects)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: drupalApplicationWithoutID.Name,
		},
	}

	t.Run("should set DrupalApplication Id", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)

		// Expecting a valid UUID should be assigned to the DrupalApplication
		drupalApp := &fnv1alpha1.DrupalApplication{}
		r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalApp)
		require.Equal(t, testhelpers.IsValidUUID(string(drupalApp.Id())), true)
	})

	t.Run("should set git repo label", func(t *testing.T) {
		res, err := r.Reconcile(req)
		// GitRepo Validation has been done seperatley
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)
	})

	t.Run("should have 0 DrupalEnvironments", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, false)

		// Expecting a DrupalApplication to set Labels accordingly
		drupalApp := &fnv1alpha1.DrupalApplication{}
		r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalApp)
		require.Equal(t, drupalApp.Status.NumEnvironments, int32(0))
	})
}

func TestApplicationController_ReconcileWithEnvironment(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	fakeObjects := []runtime.Object{
		drupalApplicationWithoutImageRepo,
		drupalEnvironmentStgWithLabel,
		drupalEnvironmentProdWithLabel,
	}

	r := buildFakeReconcile(fakeObjects)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: drupalApplicationWithoutImageRepo.Name,
		},
	}

	t.Run("should set ImageRepo field", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)

		// Expecting a ImageRepo field to be set
		drupalApp := &fnv1alpha1.DrupalApplication{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalApp)
		require.NoError(t, err)

		require.Equal(t, "881217801864.dkr.ecr.us-east-1.amazonaws.com/customer/svn-2.archteam.srvs.ahdev.co/nebula", drupalApp.Spec.ImageRepo)
	})

	t.Run("should set git repo label", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, true)

		// Expecting a DrupalApplication Status should be set correctly and will have 0 NumEnvironments
		drupalApp := &fnv1alpha1.DrupalApplication{}
		r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalApp)
		labels := drupalApp.GetLabels()

		sha := sha1.New()
		sha.Write([]byte(testGitRepo))
		hashedGitRepo := fmt.Sprintf("%x", sha.Sum(nil))
		require.Equal(t, hashedGitRepo, labels[fnv1alpha1.GitRepoLabel])
		// require.Equal(t, drupalApp.Status.NumEnvironments, int32(0))
	})

	t.Run("should have 2 DrupalEnvironments", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.Nil(t, err)
		require.Equal(t, res.Requeue, false)

		// Expecting a DrupalApplication Status should be set correctly and have 2 NumEnvironments
		drupalApp := &fnv1alpha1.DrupalApplication{}
		r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalApp)
		require.Equal(t, drupalApp.Status.NumEnvironments, int32(2))
	})

	t.Run("should have Sorted DrupalEnvironments", func(t *testing.T) {
		drupalApp := &fnv1alpha1.DrupalApplication{}
		r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalApp)

		// Expecting a DrupalApplication Status should be set correctly and in ascending order by name
		require.Equal(t, drupalApp.Status.Environments[0].Name, drupalEnvironmentProdWithLabel.Name)
		require.Equal(t, drupalApp.Status.Environments[1].Name, drupalEnvironmentStgWithLabel.Name)
	})

	t.Run("should have updated Environments in Status", func(t *testing.T) {
		drupalApp := &fnv1alpha1.DrupalApplication{}
		r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalApp)
		prodEnv := drupalApp.Status.Environments[0]

		// Expecting a DrupalApplication Status Environments should be set correctly and have all the required fields
		require.Equal(t, prodEnv.Name, drupalEnvironmentProdWithLabel.Name)
		require.Equal(t, prodEnv.Namespace, drupalEnvironmentProdWithLabel.Namespace)
		require.Equal(t, prodEnv.UID, drupalEnvironmentProdWithLabel.UID)
		require.Equal(t, prodEnv.EnvironmentID, drupalEnvironmentProdWithLabel.Labels[fnv1alpha1.EnvironmentIdLabel])

		require.True(t, goldenHelper.GoldenSpec(t, "DrupalApplication", drupalApp))
	})
}
