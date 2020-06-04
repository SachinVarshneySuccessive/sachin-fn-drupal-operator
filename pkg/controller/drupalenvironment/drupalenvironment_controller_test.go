package drupalenvironment

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"testing"
	"time"

	goldenHelper "github.com/acquia/fn-go-utils/pkg/testhelpers"
	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/argoproj/argo-rollouts/utils/conditions"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

func TestMain(m *testing.M) {
	logf.SetLogger(logf.ZapLogger(true))
	os.Exit(m.Run())
}

func TestReconcileDrupalEnvironment_Reconcile(t *testing.T) {
	_ = os.Setenv("NEWRELIC_DAEMON_ADDR", "newrelic.newrelic.svc.cluster.local:9999")

	// Set Realm and AwsRegion for tests before drupal rollout
	common.SetRealm_ForTestsOnly("TestRealm")
	common.SetAwsRegion_ForTestsOnly("TestRegion")

	// objects to track in the fake client
	objects := []runtime.Object{
		testNamespaceResource,
		drupalEnvironmentWithID,
		drupalApplicationWithID,
		siteWithID,
		testSecondSite,
		testDefaultClusterCredsSecret,
		testProdNewRelicSecret,
	}

	r := buildFakeReconcile(objects)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      drupalEnvironmentWithID.Name,
			Namespace: drupalEnvironmentWithID.Namespace,
		},
	}

	t.Run("should add DrupalCleanupFinalizer and Requeue", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verifying Finalizers has been set correctly
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)
		require.True(t, common.HasFinalizer(drupalEnvironment, drenvCleanupFinalizer))
	})

	t.Run("should have status as Syncing", func(t *testing.T) {
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)
		require.Equal(t, fnv1alpha1.DrupalEnvironmentStatusSyncing, drupalEnvironment.Status.Status)
	})

	t.Run("should setup GitRef and Requeue", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Expecting a DrupalApplication Status should be set correctly and will have 0 NumEnvironments
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		labels := drupalEnvironment.GetLabels()

		sha := sha1.New()
		sha.Write([]byte(drupalEnvironment.Spec.GitRef))
		hashedGitRef := fmt.Sprintf("%x", sha.Sum(nil))
		require.Equal(t, hashedGitRef, labels[fnv1alpha1.GitRefLabel])
	})

	t.Run("should label namespace for istio", func(t *testing.T) {
		common.SetIsIstioEnabled_ForTestsOnly(true)
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		ns := &v1.Namespace{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.Namespace}, ns)
		require.NoError(t, err)

		val, ok := ns.Labels[istioInjectionLabel]
		require.True(t, ok)
		require.Equal(t, "enabled", val)
	})

	t.Run("should remove istio label from namespace", func(t *testing.T) {
		common.SetIsIstioEnabled_ForTestsOnly(false)
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		ns := &v1.Namespace{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.Namespace}, ns)
		require.NoError(t, err)

		_, ok := ns.Labels[istioInjectionLabel]
		require.False(t, ok)
	})

	t.Run("should find parent DrupalApplication and Link to Owner with Requeue", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verifying the owners has been updated correctly
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		labels := drupalEnvironment.GetLabels()
		require.Equal(t, testAppID, labels[fnv1alpha1.ApplicationIdLabel])
	})

	t.Run("should create php-config ConfigMap and Requeue", func(t *testing.T) {
		// Verifying the php-config ConfigMap has not yet been created
		phpConfigMap := &v1.ConfigMap{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: "php-config", Namespace: testNamespace}, phpConfigMap)
		require.True(t, errors.IsNotFound(err))

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verifying the php-config ConfigMap has been updated correctly
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "php-config", Namespace: testNamespace}, phpConfigMap)
		require.NoError(t, err)

		require.True(t, goldenHelper.GoldenSpec(t, "ConfigMap", phpConfigMap))
	})

	t.Run("should create phpfpm-config ConfigMap and Requeue", func(t *testing.T) {
		// Verifying the phpfpm-config ConfigMap has not yet been created
		phpfpmConfigMap := &v1.ConfigMap{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: "phpfpm-config", Namespace: testNamespace}, phpfpmConfigMap)
		require.True(t, errors.IsNotFound(err))

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verifying the phpfpm-config ConfigMap has been updated correctly
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "phpfpm-config", Namespace: testNamespace}, phpfpmConfigMap)
		require.NoError(t, err)

		require.True(t, goldenHelper.GoldenSpec(t, "ConfigMap", phpfpmConfigMap))
	})

	t.Run("should create apache-conf-enabled ConfigMap and Requeue", func(t *testing.T) {
		// Verifying the apache-conf-enabled ConfigMap has not yet been created
		apacheConfEnabledConfigMap := &v1.ConfigMap{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: "apache-conf-enabled", Namespace: testNamespace}, apacheConfEnabledConfigMap)
		require.True(t, errors.IsNotFound(err))

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verifying the apache-conf-enabled ConfigMap has been updated correctly
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "apache-conf-enabled", Namespace: testNamespace}, apacheConfEnabledConfigMap)
		require.NoError(t, err)

		require.True(t, goldenHelper.GoldenSpec(t, "ConfigMap", apacheConfEnabledConfigMap))
	})

	t.Run("should reconcile PV and Requeue", func(t *testing.T) {
		res, err := r.Reconcile(req)
		// Expecting PV to be created successfully and requeue
		require.NoError(t, err)
		require.True(t, res.Requeue)
	})

	t.Run("should reconcile PVC and Requeue", func(t *testing.T) {
		res, err := r.Reconcile(req)
		// Expecting PVC to be created successfully and requeue
		require.NoError(t, err)
		require.True(t, res.Requeue)
	})

	t.Run("should reconcile Drupal Service and Requeue", func(t *testing.T) {
		// Verifying Drupal Service has not yet been created
		drupalService := &v1.Service{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: "drupal", Namespace: testNamespace}, drupalService)
		require.True(t, errors.IsNotFound(err))

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verifying Drupal Service has been created
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "drupal", Namespace: testNamespace}, drupalService)
		require.NoError(t, err)

		// Verifying selectors has been successfully added
		selectors := drupalService.Spec.Selector
		require.Equal(t, "drupal", selectors["app"])
	})

	t.Run("should create env-config Secret and Requeue", func(t *testing.T) {
		// Verifying the env-config Secret does not yet exist
		secret := &v1.Secret{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: "env-config", Namespace: testNamespace}, secret)
		require.True(t, errors.IsNotFound(err))

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verifying the site-settings ConfigMap has been created
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "env-config", Namespace: testNamespace}, secret)
		require.NoError(t, err)
		require.True(t, goldenHelper.Golden(t, "Decoded_sites.inc", secret.Data["sites.inc"]))
		require.True(t, goldenHelper.GoldenSpec(t, "Secret", secret))
	})

	t.Run("should reconcile Drupal Rollout and Requeue", func(t *testing.T) {
		// Verifying Drupal Rollout does not yet exist
		drupalRollout := &rolloutsv1alpha1.Rollout{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: "drupal", Namespace: testNamespace}, drupalRollout)
		require.True(t, errors.IsNotFound(err))

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.Equal(t, res.Requeue, true)

		// Verifying Drupal Rollout
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "drupal", Namespace: testNamespace}, drupalRollout)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "Rollout", drupalRollout))

		// Fake client doesn't set creation time
		drupalRollout.SetCreationTimestamp(testCreationTimestamp)
		_ = r.client.Update(context.TODO(), drupalRollout)
	})

	t.Run("should reconcile HPA", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)
	})

	t.Run("should be fully reconciled without SSHD", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.False(t, res.Requeue)
	})

	t.Run("should reconcile SSHD RBAC", func(t *testing.T) {
		// Create Authorized Keys ConfigMap
		err := r.client.Create(context.TODO(), testSshAuthorizedKeysConfigMap)
		require.NoError(t, err)

		// Should then reconcile SSHD RBAC
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		res, err = r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		sa := &v1.ServiceAccount{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: testNamespace, Name: sshdDeploymentName}, sa)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "ServiceAccount", sa))

		crb := &rbacv1.ClusterRoleBinding{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: sshdDeploymentName + "-" + testNamespace}, crb)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "ClusterRoleBinding", crb))
	})

	t.Run("should reconcile SSHD Service", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		svc := &v1.Service{}
		key := types.NamespacedName{Namespace: testNamespace, Name: sshdServiceName}
		err = r.client.Get(context.TODO(), key, svc)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "Service", svc))
	})

	t.Run("should reconcile SSHD Deployment and still have status as Syncing", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		dep := &appsv1.Deployment{}
		key := types.NamespacedName{Namespace: testNamespace, Name: sshdDeploymentName}
		err = r.client.Get(context.TODO(), key, dep)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "Deployment", dep))

		// Fake client doesn't set creation time
		dep.SetCreationTimestamp(testCreationTimestamp)
		err = r.client.Update(context.TODO(), dep)
		require.NoError(t, err)

		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)
		require.Equal(t, fnv1alpha1.DrupalEnvironmentStatusSyncing, drupalEnvironment.Status.Status)
	})

	t.Run("should have status as Deploying", func(t *testing.T) {
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)
		require.NotEqual(t, drupalEnvironment.Status.Status, fnv1alpha1.DrupalEnvironmentStatusDeploying)

		rollout := &rolloutsv1alpha1.Rollout{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: drupalRolloutName, Namespace: req.NamespacedName.Name}, rollout)
		require.NoError(t, err)

		condition := rolloutsv1alpha1.RolloutCondition{
			LastTransitionTime: metav1.Now(),
			LastUpdateTime:     metav1.Now(),
			Message:            "",
			Reason:             conditions.ReplicaSetUpdatedReason,
			Status:             v1.ConditionTrue,
			Type:               rolloutsv1alpha1.RolloutProgressing,
		}
		conditions.SetRolloutCondition(&rollout.Status, condition)
		err = r.client.Update(context.TODO(), rollout)
		require.NoError(t, err)

		// Fully reconciled at this point
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.False(t, res.Requeue)

		drupalEnvironment = &fnv1alpha1.DrupalEnvironment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)
		require.Equal(t, fnv1alpha1.DrupalEnvironmentStatusDeploying, drupalEnvironment.Status.Status)

		conditions.RemoveRolloutCondition(&rollout.Status, rolloutsv1alpha1.RolloutProgressing)
		err = r.client.Update(context.TODO(), rollout)
		require.NoError(t, err)
	})

	t.Run("should have status as Synced", func(t *testing.T) {
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)
		require.NotEqual(t, drupalEnvironment.Status.Status, fnv1alpha1.DrupalEnvironmentStatusSynced)

		rollout := &rolloutsv1alpha1.Rollout{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: drupalRolloutName, Namespace: req.NamespacedName.Name}, rollout)
		require.NoError(t, err)

		progressingCondition := rolloutsv1alpha1.RolloutCondition{
			LastTransitionTime: metav1.Now(),
			LastUpdateTime:     metav1.Now(),
			Message:            "",
			Reason:             conditions.NewRSAvailableReason,
			Status:             v1.ConditionTrue,
			Type:               rolloutsv1alpha1.RolloutProgressing,
		}
		conditions.SetRolloutCondition(&rollout.Status, progressingCondition)

		availableCondition := rolloutsv1alpha1.RolloutCondition{
			LastTransitionTime: metav1.Now(),
			LastUpdateTime:     metav1.Now(),
			Message:            "",
			Reason:             conditions.AvailableReason,
			Status:             v1.ConditionTrue,
			Type:               rolloutsv1alpha1.RolloutAvailable,
		}
		conditions.SetRolloutCondition(&rollout.Status, availableCondition)
		err = r.client.Update(context.TODO(), rollout)
		require.NoError(t, err)

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.False(t, res.Requeue)

		drupalEnvironment = &fnv1alpha1.DrupalEnvironment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)
		require.Equal(t, fnv1alpha1.DrupalEnvironmentStatusSynced, drupalEnvironment.Status.Status)

		conditions.RemoveRolloutCondition(&rollout.Status, rolloutsv1alpha1.RolloutProgressing)
		conditions.RemoveRolloutCondition(&rollout.Status, rolloutsv1alpha1.RolloutAvailable)
		err = r.client.Update(context.TODO(), rollout)
		require.NoError(t, err)
	})

	t.Run("should update the php-config ConfigMap and Requeue", func(t *testing.T) {
		// Fetch Environment
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)

		// Updating Specs
		drupalEnvironment.Spec.Phpfpm.OpcacheMemoryLimitMiB = 100
		err = r.client.Update(context.TODO(), drupalEnvironment)
		require.NoError(t, err)

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verify the php-config ConfigMap was updated
		phpConfigMap := &v1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "php-config", Namespace: testNamespace}, phpConfigMap)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "ConfigMap", phpConfigMap))

		// Verify the hash value is updated to relaunch drupal Pods
		before := &rolloutsv1alpha1.Rollout{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: drupalRolloutName, Namespace: req.NamespacedName.Name}, before)
		require.NoError(t, err)

		res, err = r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		after := &rolloutsv1alpha1.Rollout{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: drupalRolloutName, Namespace: req.NamespacedName.Name}, after)
		require.NoError(t, err)

		annoBefore := before.Spec.Template.ObjectMeta.Annotations[fnv1alpha1.ConfigHashAnnotation]
		annoAfter := after.Spec.Template.ObjectMeta.Annotations[fnv1alpha1.ConfigHashAnnotation]

		require.NotEmpty(t, annoAfter)
		require.NotEqual(t, annoBefore, annoAfter)
	})

	t.Run("php-fpm configuration for pre-7.3", func(t *testing.T) {
		// Fetch Environment
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)

		// Changing PHP version to 7.2 to verify that config specific to version 7.3+ does not apply
		drupalEnvironment.Spec.Phpfpm.Tag = "7.2"
		err = r.client.Update(context.TODO(), drupalEnvironment)
		require.NoError(t, err)

		// Reconcile to update the phpfpm-config ConfigMap
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verifying the phpfpm-config ConfigMap was updated
		phpfpmConfigMap := &v1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "phpfpm-config", Namespace: testNamespace}, phpfpmConfigMap)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "PhpfpmConfigMapWithPHPLessThan73", phpfpmConfigMap))

		// Making sure Rollout gets updated
		res, err = r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		drupalRollout := &rolloutsv1alpha1.Rollout{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "drupal", Namespace: testNamespace}, drupalRollout)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "Rollout", drupalRollout))

		// Making sure SSH Deployment gets updated
		res, err = r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		sshDeployment := &appsv1.Deployment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: sshdDeploymentName, Namespace: testNamespace}, sshDeployment)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "SSHDeployment", sshDeployment))

		// Fully reconciled
		res, err = r.Reconcile(req)
		require.NoError(t, err)
		require.False(t, res.Requeue)
	})

	t.Run("should delete DrupalEnvironment", func(t *testing.T) {
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)
		require.NotEqual(t, drupalEnvironment.Status.Status, fnv1alpha1.DrupalEnvironmentStatusDeleting)

		drupalEnvironment.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
		err = r.client.Update(context.TODO(), drupalEnvironment)
		require.NoError(t, err)

		// Removing of PV
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Removing of SSH RBAC
		res, err = r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		ctx := context.TODO()
		crb := &rbacv1.ClusterRoleBinding{}
		err = r.client.Get(ctx, types.NamespacedName{Name: "sshd-" + req.Namespace}, crb)
		require.True(t, errors.IsNotFound(err))

		// Reconcile complete
		res, err = r.Reconcile(req)
		require.NoError(t, err)
		require.False(t, res.Requeue)
	})

	t.Run("should have status as deleting", func(t *testing.T) {
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: req.NamespacedName.Name}, drupalEnvironment)
		require.NoError(t, err)
		require.Equal(t, fnv1alpha1.DrupalEnvironmentStatusDeleting, drupalEnvironment.Status.Status)
	})
}

func TestReconcileDrupalEnvironment_Reconcile_UnknownNamespace(t *testing.T) {
	// objects to track in the fake client
	objects := []runtime.Object{
		drupalEnvironmentWithoutID,
	}

	r := buildFakeReconcile(objects)

	// mock request to simulate Reconcile() being called on an event for a watched resource
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      drupalEnvironmentWithoutID.Name,
			Namespace: drupalEnvironmentWithoutID.Namespace,
		},
	}

	_, _ = r.Reconcile(req)      // migrate
	_, _ = r.Reconcile(req)      // set id
	_, _ = r.Reconcile(req)      // add finalizer
	_, _ = r.Reconcile(req)      // label env
	res, err := r.Reconcile(req) // try to label namespace
	require.Error(t, err)
	require.False(t, res.Requeue)
}

func TestReconcileDrupalEnvironment_Reconcile_NonProdValues(t *testing.T) {
	nonProdSshConfigMap := testSshAuthorizedKeysConfigMap.DeepCopy()
	nonProdSshConfigMap.Namespace = testNonProdNamespace

	// objects to track in the fake client
	objects := []runtime.Object{
		drupalEnvironmentWithNonProdValues,
		testNonProdNamespaceResource,
		drupalApplicationWithID,
		testNonProdNewRelicSecret,
		nonProdSshConfigMap,
	}

	r := buildFakeReconcile(objects)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      drupalEnvironmentWithNonProdValues.Name,
			Namespace: drupalEnvironmentWithNonProdValues.Namespace,
		},
	}

	t.Run("should fully reconcile", func(t *testing.T) {
		fullyReconciled := false
		for i := 0; i < 20; i++ {
			res, err := r.Reconcile(req)

			// Expecting Non Prod values in golden files
			require.NoError(t, err)
			if !res.Requeue && res.RequeueAfter == 0 {
				fullyReconciled = true
				break
			}
		}
		require.True(t, fullyReconciled)

		drupalRollout := &rolloutsv1alpha1.Rollout{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: "drupal", Namespace: req.Namespace}, drupalRollout)
		require.NoError(t, err)

		require.True(t, goldenHelper.GoldenSpec(t, "Rollout", drupalRollout))
	})

	t.Run("config file ConfigMaps should be correct", func(t *testing.T) {
		// Verify the Apache ConfigMap
		configMap := &v1.ConfigMap{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: "apache-conf-enabled", Namespace: testNonProdNamespace}, configMap)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "apache-conf-enabled", configMap))

		// Verify the php-config ConfigMap
		configMap = &v1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "php-config", Namespace: testNonProdNamespace}, configMap)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "php-config", configMap))

		// Verify the phpfpm-config ConfigMap
		configMap = &v1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "phpfpm-config", Namespace: testNonProdNamespace}, configMap)
		require.NoError(t, err)
		require.True(t, goldenHelper.GoldenSpec(t, "phpfpm-config", configMap))
	})
}

func TestReconcileDrupalEnvironment_ReconcileWithoutEnvID(t *testing.T) {
	// Have to set it because - GenerateName doesn't work with fake client.
	// drupalEnvironment.SetTargetNamespace("generated-namespace")

	// objects to track in the fake client
	objects := []runtime.Object{
		drupalEnvironmentWithoutID,
	}

	r := buildFakeReconcile(objects)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: drupalEnvironmentWithoutID.Name,
		},
	}

	_, _ = r.Reconcile(req) // migrate

	t.Run("should set DrupalEnvironment ID and Requeue", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verifying that valid UUID has been assigned to drupalEnvironment
		drupalEnvironment := &fnv1alpha1.DrupalEnvironment{}
		r.client.Get(context.TODO(), types.NamespacedName{
			Name: req.NamespacedName.Name,
		}, drupalEnvironment)
		require.True(t, testhelpers.IsValidUUID(string(drupalEnvironment.Id())))
	})
}

func TestReconcileDrupalEnvironment_ReconcileWithoutDrupalApplication(t *testing.T) {
	// objects to track in the fake client
	objects := []runtime.Object{
		testNamespaceResource,
		drupalEnvironmentWithID,
	}

	r := buildFakeReconcile(objects)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      drupalEnvironmentWithID.Name,
			Namespace: drupalEnvironmentWithID.Namespace,
		},
	}

	res, err := r.Reconcile(req)
	// expecting finalizer to be added, and must requeue
	// there is separate test for finalizers
	require.NoError(t, err)
	require.True(t, res.Requeue)

	// expecting gitRef to set, and must requeue
	// there is a separate check for GitRef
	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.True(t, res.Requeue)

	t.Run("should not find DrupalApplication and add to Requeue", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.Equal(t, 10*time.Second, res.RequeueAfter)
	})
}

func Test_getSshUsername(t *testing.T) {
	// objects to track in the fake client
	objects := []runtime.Object{
		testNamespaceResource,
		drupalEnvironmentWithID,
		testSshAuthorizedKeysConfigMap,
	}

	rh := requestHandler{
		reconciler: buildFakeReconcile(objects),
		env:        drupalEnvironmentWithID,
	}
	username, err := rh.getSSHUsername()
	require.NoError(t, err)
	require.Equal(t, testSshUsername, username)
}

func TestStageMigration(t *testing.T) {
	tests := []struct {
		name          string
		drenvName     string
		drenvStage    string
		expectedStage string
	}{
		{
			"migrate stage dev",
			"envdev",
			"",
			"dev",
		},
		{
			"migrate stage test",
			"envtest",
			"",
			"test",
		},
		{
			"migrate stage prod",
			"env",
			"",
			"prod",
		},
		{
			"don't migrate if stage already set",
			"env",
			"dev",
			"dev",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := drupalEnvironmentWithID.DeepCopy()
			delete(env.Labels, fnv1alpha1.VersionLabel)
			env.Name = test.drenvName
			env.Spec.Stage = test.drenvStage

			objects := []runtime.Object{
				testNamespaceResource,
				drupalApplicationWithID,
				env,
			}

			r := buildFakeReconcile(objects)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      env.Name,
					Namespace: env.Namespace,
				},
			}

			res, err := r.Reconcile(req)
			require.NoError(t, err)
			require.True(t, res.Requeue)

			// Verifying migration
			found := &fnv1alpha1.DrupalEnvironment{}
			err = r.client.Get(context.TODO(), req.NamespacedName, found)
			require.NoError(t, err)
			require.Equal(t, found.SpecVersion(), found.Labels[fnv1alpha1.VersionLabel])
			require.Equal(t, test.expectedStage, found.Spec.Stage)
		})
	}
}
