package command

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-go-utils/pkg/testhelpers"
)

// Test_FindContainerByName tests findContainerByName()
func Test_FindContainerByName(t *testing.T) {
	c1 := corev1.Container{Name: "container-1"}
	c2 := corev1.Container{Name: "container-2"}
	c3 := corev1.Container{Name: "container-3"}

	cs := []corev1.Container{c1, c2, c3}

	// Find all containers in the array
	for _, c := range cs {
		found := findContainerByName(cs, c.Name)
		require.NotNil(t, found)
		require.Equal(t, c.Name, found.Name)
	}

	// Fail to find a container that's not in the array
	found := findContainerByName(cs, "foo")
	require.Nil(t, found)
}

// Test_JobName tests findContainerByName()
func Test_JobName(t *testing.T) {
	name := jobName(defaultCommandOnSite)
	require.Equal(t, "command-"+defaultCommandOnSite.Name, name)
}

// Test_UnsupportedTargetRef tests proper handling when targetRef specifies an unsupported API Group/Version/Kind.
func Test_UnsupportedTargetRef(t *testing.T) {
	invalidCommand := &v1alpha1.Command{}
	defaultCommandOnSite.DeepCopyInto(invalidCommand)

	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// mock request to simulate Reconcile() being called on an event for a watched resource
	commandKey := types.NamespacedName{Name: defaultCommandOnSite.Name, Namespace: defaultCommandOnSite.Namespace}
	req := reconcile.Request{NamespacedName: commandKey}

	requeueAfterResult := reconcile.Result{RequeueAfter: 60 * time.Second}

	t.Run("Verify failure on invalid API Version", func(t *testing.T) {
		invalidCommand := &v1alpha1.Command{}
		defaultCommandOnSite.DeepCopyInto(invalidCommand)

		invalidCommand.Spec.TargetRef.APIVersion = "foo/bar/baz"

		// objects to track in the fake client
		objects := []runtime.Object{
			drupalApplication,
			drupalEnvironment,
			site,
			drupalPod,
			invalidCommand,
		}

		r := BuildFakeReconcile(objects)

		// Reconcile should succeed, with delayed requeue
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.Equal(t, requeueAfterResult, res)
	})

	t.Run("Verify failure on unsupported API Group", func(t *testing.T) {
		invalidCommand := &v1alpha1.Command{}
		defaultCommandOnSite.DeepCopyInto(invalidCommand)

		invalidCommand.Spec.TargetRef.APIVersion = "foobar/v1"

		// objects to track in the fake client
		objects := []runtime.Object{
			drupalApplication,
			drupalEnvironment,
			site,
			drupalPod,
			invalidCommand,
		}

		r := BuildFakeReconcile(objects)

		// Reconcile should succeed, with delayed requeue
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.Equal(t, requeueAfterResult, res)
	})

	t.Run("Verify failure on unsupported API Version", func(t *testing.T) {
		invalidCommand := &v1alpha1.Command{}
		defaultCommandOnSite.DeepCopyInto(invalidCommand)

		invalidCommand.Spec.TargetRef.APIVersion = "fnresources.acquia.io/v1alpha99"

		// objects to track in the fake client
		objects := []runtime.Object{
			drupalApplication,
			drupalEnvironment,
			site,
			drupalPod,
			invalidCommand,
		}

		r := BuildFakeReconcile(objects)

		// Reconcile should succeed, with delayed requeue
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.Equal(t, requeueAfterResult, res)
	})

	t.Run("Verify failure on unsupported Kind", func(t *testing.T) {
		invalidCommand := &v1alpha1.Command{}
		defaultCommandOnSite.DeepCopyInto(invalidCommand)

		invalidCommand.Spec.TargetRef.APIVersion = "fnresources.acquia.io/v1alpha1"
		invalidCommand.Spec.TargetRef.Kind = "FooBar"

		// objects to track in the fake client
		objects := []runtime.Object{
			drupalApplication,
			drupalEnvironment,
			site,
			drupalPod,
			invalidCommand,
		}

		r := BuildFakeReconcile(objects)

		// Reconcile should succeed, with delayed requeue
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.Equal(t, requeueAfterResult, res)
	})
}

// Test_ReconcileDefaultCommand reconciles defaultCommandOnSite to completion, verifying the expected
// resources were created by the controller, with the expected field values. It then verifies that an update to the
// command doesn't update the related Job.
func Test_ReconcileDefaultCommand(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	objects := []runtime.Object{
		drupalApplication,
		drupalEnvironment,
		site,
		drupalPod,
		defaultCommandOnSite,
	}

	r := BuildFakeReconcile(objects)

	// mock request to simulate Reconcile() being called on an event for a watched resource
	commandKey := types.NamespacedName{Name: defaultCommandOnSite.Name, Namespace: defaultCommandOnSite.Namespace}
	req := reconcile.Request{NamespacedName: commandKey}

	// Reconcile to assign ownership of Command to targetRef
	res, err := r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{Requeue: true}, res)

	found := &v1alpha1.Command{}
	err = r.client.Get(context.TODO(), commandKey, found)
	require.NoError(t, err)
	require.Equal(t, site.UID, found.OwnerReferences[0].UID)
	require.True(t, *found.OwnerReferences[0].Controller)

	// Reconcile to create Job resource
	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{Requeue: true}, res)

	job := &batchv1.Job{}
	err = r.client.Get(
		context.TODO(),
		types.NamespacedName{Name: "command-" + defaultCommandOnSite.Name, Namespace: defaultCommandOnSite.Namespace},
		job)
	require.NoError(t, err)

	require.True(t, testhelpers.GoldenSpec(t, "basicCommandJobSpec", job.Spec))

	// Reconcile to verify no changes or requeueing
	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, res)

	// Verify that Update()-ing a one-time Command does not change the Job
	found.Spec.Command = []string{"foo", "bar"}
	err = r.client.Update(context.TODO(), found)
	require.NoError(t, err)

	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, res)

	err = r.client.Get(
		context.TODO(),
		types.NamespacedName{Name: "command-" + defaultCommandOnSite.Name, Namespace: defaultCommandOnSite.Namespace},
		job)
	require.NoError(t, err)
	require.Equal(t, defaultCommandOnSite.Spec.Command, job.Spec.Template.Spec.Containers[0].Command)

	// Explicitly set the Status of the child Job, and verify it is copied to the Command on reconcile.
	job.Status = batchv1.JobStatus{
		Conditions: []batchv1.JobCondition{
			{
				Type:               batchv1.JobComplete,
				LastProbeTime:      testTimeNow,
				LastTransitionTime: testTimeNow,
				Status:             "True",
			},
		},
		StartTime:      &testTimeThen,
		CompletionTime: &testTimeNow,
		Active:         1,
		Succeeded:      2,
		Failed:         3,
	}
	err = r.client.Update(context.TODO(), job)
	require.NoError(t, err)

	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, res)

	err = r.client.Get(context.TODO(), commandKey, found)
	require.NoError(t, err)
	require.True(t, testhelpers.GoldenSpec(t, "basicCommandJobStatus", found.Status.Job))
	require.Empty(t, found.Status.CronJob)
}

// Test_ReconcileRootCommand reconciles rootCommandOnEnv to completion, verifying the expected
// resources were created by the controller, with the expected field values. It then verifies that an update to the
// command doesn't update the related Job.
func Test_ReconcileRootCommand(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	objects := []runtime.Object{
		drupalApplication,
		drupalEnvironment,
		site,
		drupalPod,
		rootCommandOnEnv,
	}

	r := BuildFakeReconcile(objects)

	// mock request to simulate Reconcile() being called on an event for a watched resource
	commandKey := types.NamespacedName{Name: rootCommandOnEnv.Name, Namespace: rootCommandOnEnv.Namespace}
	req := reconcile.Request{NamespacedName: commandKey}

	// Reconcile to assign ownership of Command to targetRef
	res, err := r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{Requeue: true}, res)

	found := &v1alpha1.Command{}
	err = r.client.Get(context.TODO(), commandKey, found)
	require.NoError(t, err)
	require.Equal(t, drupalEnvironment.UID, found.OwnerReferences[0].UID)
	require.True(t, *found.OwnerReferences[0].Controller)

	// Reconcile to create Job resource
	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{Requeue: true}, res)

	job := &batchv1.Job{}
	err = r.client.Get(
		context.TODO(),
		types.NamespacedName{Name: "command-" + rootCommandOnEnv.Name, Namespace: rootCommandOnEnv.Namespace},
		job)
	require.NoError(t, err)

	require.True(t, testhelpers.GoldenSpec(t, "rootCommandJobSpec", job.Spec))

	// Reconcile to verify no changes or requeueing
	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, res)
}

// Test_ReconcileCronCommand reconciles defaultCronCommand to completion, verifying the expected
// resources were created by the controller, with the expected field values. It then verifies that an update to the
// command does update the related CronJob.
func Test_ReconcileCronCommand(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	objects := []runtime.Object{
		drupalApplication,
		drupalEnvironment,
		site,
		drupalPod,
		defaultCronCommand,
	}

	r := BuildFakeReconcile(objects)

	// mock request to simulate Reconcile() being called on an event for a watched resource
	commandKey := types.NamespacedName{Name: defaultCronCommand.Name, Namespace: defaultCronCommand.Namespace}
	req := reconcile.Request{NamespacedName: commandKey}

	// Reconcile to assign ownership of Command to targetRef
	res, err := r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{Requeue: true}, res)

	found := &v1alpha1.Command{}
	err = r.client.Get(context.TODO(), commandKey, found)
	require.NoError(t, err)
	require.Equal(t, site.UID, found.OwnerReferences[0].UID)
	require.True(t, *found.OwnerReferences[0].Controller)

	// Reconcile to create CronJob resource
	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{Requeue: true}, res)

	cron := &batchv1beta1.CronJob{}
	err = r.client.Get(
		context.TODO(),
		types.NamespacedName{Name: "command-" + defaultCronCommand.Name, Namespace: defaultCronCommand.Namespace},
		cron)
	require.NoError(t, err)

	require.True(t, testhelpers.GoldenSpec(t, "cronCommandCronSpec", cron.Spec))

	// Explicitly set CreationTimestamp on CronJob, so controllerutil.CreateOrUpdate() works properly
	cron.CreationTimestamp = testCreationTimestamp
	err = r.client.Update(context.TODO(), cron)
	require.NoError(t, err)

	// Reconcile to verify no changes or requeueing
	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, res)

	// Verify that Update()-ing a scheduled Command does change the CronJob
	found.Spec.Command = []string{"foo", "bar"}
	err = r.client.Update(context.TODO(), found)
	require.NoError(t, err)

	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{Requeue: true}, res)

	err = r.client.Get(
		context.TODO(),
		types.NamespacedName{Name: "command-" + defaultCronCommand.Name, Namespace: defaultCronCommand.Namespace},
		cron)
	require.NoError(t, err)
	require.Equal(t, found.Spec.Command, cron.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command)

	// Explicitly set the Status of the child CronJob, and verify it is copied to the Command on reconcile.
	cron.Status = batchv1beta1.CronJobStatus{
		LastScheduleTime: &testTimeNow,
	}
	err = r.client.Update(context.TODO(), cron)
	require.NoError(t, err)

	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, res)

	err = r.client.Get(context.TODO(), commandKey, found)
	require.NoError(t, err)
	require.True(t, testhelpers.GoldenSpec(t, "cronJobStatus", found.Status.CronJob))
	require.Empty(t, found.Status.Job)
}

// Test_ReconcileRootCronCommand reconciles rootCronCommand to completion, verifying the expected
// resources were created by the controller, with the expected field values. It then verifies that an update to the
// command does update the related CronJob.
func Test_ReconcileRootCronCommand(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	objects := []runtime.Object{
		drupalApplication,
		drupalEnvironment,
		site,
		drupalPod,
		rootCronCommand,
	}

	r := BuildFakeReconcile(objects)

	// mock request to simulate Reconcile() being called on an event for a watched resource
	commandKey := types.NamespacedName{Name: rootCronCommand.Name, Namespace: rootCronCommand.Namespace}
	req := reconcile.Request{NamespacedName: commandKey}

	// Reconcile to assign ownership of Command to targetRef
	res, err := r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{Requeue: true}, res)

	found := &v1alpha1.Command{}
	err = r.client.Get(context.TODO(), commandKey, found)
	require.NoError(t, err)
	require.Equal(t, site.UID, found.OwnerReferences[0].UID)
	require.True(t, *found.OwnerReferences[0].Controller)

	// Reconcile to create CronJob resource
	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{Requeue: true}, res)

	cron := &batchv1beta1.CronJob{}
	err = r.client.Get(
		context.TODO(),
		types.NamespacedName{Name: "command-" + rootCronCommand.Name, Namespace: rootCronCommand.Namespace},
		cron)
	require.NoError(t, err)

	require.True(t, testhelpers.GoldenSpec(t, "rootCronCommandCronSpec", cron.Spec))

	// Explicitly set CreationTimestamp on CronJob, so controllerutil.CreateOrUpdate() works properly
	cron.CreationTimestamp = testCreationTimestamp
	err = r.client.Update(context.TODO(), cron)
	require.NoError(t, err)

	// Reconcile to verify no changes or requeueing
	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, res)

	// Verify that Update()-ing a scheduled Command does change the CronJob
	found.Spec.Command = []string{"foo", "bar"}
	err = r.client.Update(context.TODO(), found)
	require.NoError(t, err)

	res, err = r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{Requeue: true}, res)

	err = r.client.Get(
		context.TODO(),
		types.NamespacedName{Name: "command-" + defaultCronCommand.Name, Namespace: defaultCronCommand.Namespace},
		cron)
	require.NoError(t, err)
	require.Equal(t, found.Spec.Command, cron.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command)
}

// Test_NoDrupalPods verifies failure if no matching 'drupal' Pods found.
func Test_NoDrupalPods(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	requeueAfterResult := reconcile.Result{RequeueAfter: 60 * time.Second}

	// objects to track in the fake client
	objects := []runtime.Object{
		drupalApplication,
		drupalEnvironment,
		site,
		defaultCommandOnSite,
	}

	r := BuildFakeReconcile(objects)

	// Mock request to simulate Reconcile() being called on an event for a watched resource
	commandKey := types.NamespacedName{Name: defaultCommandOnSite.Name, Namespace: defaultCommandOnSite.Namespace}
	req := reconcile.Request{NamespacedName: commandKey}

	// Reconcile should result in a delayed requeue
	res, err := r.Reconcile(req)
	require.NoError(t, err)
	require.Equal(t, requeueAfterResult, res)
}

// Test_NoMatchingContainer verifies failure if container needed for templating not found.
func Test_NoMatchingContainer(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// Mock request to simulate Reconcile() being called on an event for a watched resource
	commandKey := types.NamespacedName{Name: defaultCommandOnSite.Name, Namespace: defaultCommandOnSite.Namespace}
	req := reconcile.Request{NamespacedName: commandKey}

	requeueAfterResult := reconcile.Result{RequeueAfter: 60 * time.Second}

	t.Run("Verify failure on no 'php-fpm' Container", func(t *testing.T) {
		// Change name of the php-fpm container on the mock object, so it no longer matches.
		invalidDrupalPod := &corev1.Pod{}
		drupalPod.DeepCopyInto(invalidDrupalPod)
		invalidDrupalPod.Spec.Containers[0].Name = "foo"

		// objects to track in the fake client
		objects := []runtime.Object{
			drupalApplication,
			drupalEnvironment,
			site,
			invalidDrupalPod,
			defaultCommandOnSite,
		}

		r := BuildFakeReconcile(objects)

		// Reconcile should result in a delayed requeue
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.Equal(t, requeueAfterResult, res)
	})

	t.Run("Verify failure on no 'code-copy' Container", func(t *testing.T) {
		// Change name of the code-copy container on the mock object, so it no longer matches.
		invalidDrupalPod := &corev1.Pod{}
		drupalPod.DeepCopyInto(invalidDrupalPod)
		invalidDrupalPod.Spec.InitContainers[0].Name = "foo"

		// objects to track in the fake client
		objects := []runtime.Object{
			drupalApplication,
			drupalEnvironment,
			site,
			invalidDrupalPod,
			defaultCommandOnSite,
		}

		r := BuildFakeReconcile(objects)

		// Reconcile should result in a delayed requeue
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.Equal(t, requeueAfterResult, res)
	})
}
