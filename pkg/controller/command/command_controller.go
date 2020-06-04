package command

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-test/deep"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	fnresources "github.com/acquia/fn-drupal-operator/pkg/apis"
	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"
)

var log = logf.Log.WithName("controller_command")

// Add creates a new Command Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	scheme := mgr.GetScheme()
	if err := fnresources.AddToScheme(scheme); err != nil {
		panic(err)
	}
	return &ReconcileCommand{client: mgr.GetClient(), scheme: scheme}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("command-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Command
	err = c.Watch(&source.Kind{Type: &fnv1alpha1.Command{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Jobs and requeue the owner Command
	err = common.WatchOwned(c, &fnv1alpha1.Command{}, []runtime.Object{
		&batchv1.Job{},
		&batchv1beta1.CronJob{},
	})
	return err
}

// blank assignment to verify that ReconcileCommand implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileCommand{}

// ReconcileCommand reconciles a Command object
type ReconcileCommand struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

type requestHandler struct {
	r      *ReconcileCommand
	logger logr.Logger

	cmd *fnv1alpha1.Command
	jobParams
}

// jobParams contain's the desired specs for the Command's Job/CronJob and Pod. They will be derived from the
// targetRef's Pods.
type jobParams struct {
	container corev1.Container
	volumes   []corev1.Volume
	labels    map[string]string
}

// Reconcile reads that state of the cluster for a Command object and makes changes based on the state read
// and what is in the Command.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCommand) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	logger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	logger.V(1).Info("Reconciling Command")

	// Fetch the Command instance
	cmd := &fnv1alpha1.Command{}
	err = r.client.Get(context.TODO(), request.NamespacedName, cmd)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, likely deleted after reconcile request, so ignore.
			return result, nil
		}
		return
	}

	rh := requestHandler{r: r, cmd: cmd, logger: logger}

	result, err = rh.doReconcile(request)

	// The status on the Command object gets updated during the reconcile process, so commit it now.
	if errStatus := r.client.Status().Update(context.TODO(), rh.cmd); errStatus != nil {
		log.Error(errStatus, "Failed to update Status")
		if err == nil {
			err = errStatus
		}
	}
	return
}

func (rh *requestHandler) doReconcile(request reconcile.Request) (result reconcile.Result, err error) {
	// Check if the Job/CronJob can be reconciled
	var needsReconcile bool
	needsReconcile, err = rh.needsReconcile()
	if err != nil || !needsReconcile {
		return
	}

	// Generate jobParams to be used for this request
	var target metav1.Object
	rh.jobParams, target, err = rh.generateJobParams()
	if err != nil {
		rh.logger.Error(err, "Failed to generate jobParams")
		result.RequeueAfter = 60 * time.Second
		return result, nil
	}

	// Assign ownership of this Command to target
	var changed bool
	if changed, err = common.LinkToOwner(target, rh.cmd, rh.r.scheme); err != nil {
		rh.logger.Info("Command already has a different Controller Owner")
	} else if changed {
		err = rh.r.client.Update(context.TODO(), rh.cmd)
		result.Requeue = true
		return
	}

	// Create or Update the Job/CronJob
	if rh.isCommandScheduled() {
		return rh.reconcileCronJob()
	} else if len(rh.cmd.Status.Job.Conditions) == 0 {
		// Only reconcile if Job has not yet started, in case the Job is missing because it was deleted
		return rh.reconcileJob()
	}

	return
}

func (rh *requestHandler) isCommandScheduled() bool {
	return rh.cmd.Spec.Schedule != ""
}

// needsReconcile checks if the Job/CronJob can be further reconciled. In the case of Job, don't reconcile if it
// exists (since it doesn't make sense to modify)
func (rh *requestHandler) needsReconcile() (needsReconcile bool, err error) {
	key := types.NamespacedName{Name: jobName(rh.cmd), Namespace: rh.cmd.Namespace}

	if rh.isCommandScheduled() {
		cronJob := &batchv1beta1.CronJob{}
		err = rh.r.client.Get(context.TODO(), key, cronJob)
		if err == nil {
			// Update our status while we have the object handy
			rh.cmd.Status.CronJob = cronJob.Status
		} else if !errors.IsNotFound(err) {
			// Return if an unexpected error occurred
			return
		}
	} else {
		job := &batchv1.Job{}
		err = rh.r.client.Get(context.TODO(), key, job)
		if err == nil {
			// Update our status while we have the object handy
			rh.cmd.Status.Job = job.Status
		}
		if !errors.IsNotFound(err) {
			// Return if Job already exists (since it can't really be modified), or an unexpected error occurred
			return
		}
	}
	return true, nil
}

func (rh *requestHandler) generateJobParams() (jobParams jobParams, target metav1.Object, err error) {
	// Call the API Group handler for this targetRef's GroupVersion
	var gv schema.GroupVersion
	if gv, err = schema.ParseGroupVersion(rh.cmd.Spec.TargetRef.APIVersion); err != nil {
		rh.logger.Error(err, "targetRef has invalid APIVersion")
		return
	}

	switch gv.Group {
	case fnv1alpha1.SchemeGroupVersion.Group:
		return rh.generateJobParamsFromFnResources(gv.Version)
	default:
		err = fmt.Errorf("unsupported targetRef API Group: '%v'", gv.Group)
		return
	}
}

func (rh *requestHandler) reconcileCronJob() (result reconcile.Result, err error) {
	cronJob := &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName(rh.cmd),
			Namespace: rh.cmd.Namespace,
		},
	}

	var op controllerutil.OperationResult
	op, err = controllerutil.CreateOrUpdate(context.TODO(), rh.r.client, cronJob, func() error {
		desiredSpec := rh.cronJobSpec()

		if cronJob.CreationTimestamp.IsZero() {
			// Creating new CronJob. Set Command instance as the owner and controller (will never error since CronJob is new)
			_ = controllerutil.SetControllerReference(rh.cmd, cronJob, rh.r.scheme)
		}

		if diff := deep.Equal(cronJob.Spec, desiredSpec); diff != nil {
			rh.logger.Info("CronJob Spec needs update", "current != desired", diff)
			cronJob.Spec = desiredSpec
		}

		cronJob.Labels = common.MergeLabels(cronJob.Labels, rh.jobParams.labels)

		return nil
	})
	if err != nil {
		return
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Successfully reconciled CronJob", "Operation", op)
		result.Requeue = true
	}
	return
}

func (rh *requestHandler) reconcileJob() (result reconcile.Result, err error) {
	job := rh.newJob()

	// Set Command instance as the owner and controller (will never error since Job is new)
	_ = controllerutil.SetControllerReference(rh.cmd, job, rh.r.scheme)

	err = rh.r.client.Create(context.TODO(), job)
	if err != nil && errors.IsAlreadyExists(err) {
		// Allow "already exists" error
		rh.logger.Info("Job already exists", "Name", job.Name)
		return result, nil
	}

	rh.logger.Info("Created Job", "Name", job.Name)
	result.Requeue = true
	return
}

func (rh *requestHandler) newJob() *batchv1.Job {
	rootUser := int64(0)
	completions := int32(1)
	terminationGracePeriod := int64(corev1.DefaultTerminationGracePeriodSeconds)

	activeDeadlineSeconds := int64(3600) // By default, Job has one hour to complete or it will be killed
	if rh.cmd.Spec.ActiveDeadlineSeconds != nil {
		activeDeadlineSeconds = *rh.cmd.Spec.ActiveDeadlineSeconds
	}

	restartPolicy := rh.cmd.Spec.RestartPolicy
	if restartPolicy == "" {
		restartPolicy = corev1.RestartPolicyNever
	}

	customerContainer := rh.jobParams.container
	customerContainer.Command = rh.cmd.Spec.Command
	customerContainer.Name = "main"
	customerContainer.LivenessProbe = nil
	customerContainer.ReadinessProbe = nil

	if rh.cmd.Spec.Image != "" {
		customerContainer.Image = rh.cmd.Spec.Image
	}

	// Optionally override resource requests
	if rh.cmd.Spec.Resources != nil {
		customerContainer.Resources = *rh.cmd.Spec.Resources
	}

	// Add additional env vars, volumes, and mounts
	for _, envVar := range rh.cmd.Spec.AdditionalEnvVars {
		customerContainer.Env = append(customerContainer.Env, envVar)
	}

	for _, m := range rh.cmd.Spec.AdditionalVolumeMounts {
		customerContainer.VolumeMounts = append(customerContainer.VolumeMounts, m)
	}

	volumes := rh.jobParams.volumes
	for _, v := range rh.cmd.Spec.AdditionalVolumes {
		volumes = append(volumes, v)
	}

	// Run as root if specified
	var securityContext corev1.PodSecurityContext
	if rh.cmd.Spec.RunAsRoot {
		securityContext.RunAsUser = &rootUser
	}

	// Set additional labels
	labels := rh.jobParams.labels
	for key, val := range rh.cmd.Spec.AdditionalLabels {
		labels[key] = val
	}

	if rh.cmd.Spec.TerminationGracePeriodSeconds != nil {
		terminationGracePeriod = *rh.cmd.Spec.TerminationGracePeriodSeconds
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName(rh.cmd),
			Namespace: rh.cmd.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &rh.cmd.Spec.Retries,
			Completions:           &completions,
			ActiveDeadlineSeconds: &activeDeadlineSeconds,

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{},
					Labels:            rh.jobParams.labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                 restartPolicy,
					InitContainers:                rh.cmd.Spec.InitContainers,
					Containers:                    []corev1.Container{customerContainer},
					Volumes:                       volumes,
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					SecurityContext:               &securityContext,
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/worker": "true",
					},
					DNSPolicy:     corev1.DNSClusterFirst,
					SchedulerName: corev1.DefaultSchedulerName,
				},
			},
		},
	}
}

func (rh *requestHandler) cronJobSpec() batchv1beta1.CronJobSpec {
	historyLimit := int32(2)
	startingDeadlineSeconds := int64(900)

	suspend := false
	if rh.cmd.Spec.Suspend {
		suspend = true
	}

	// Default concurrencyPolicy is Forbid
	concurrencyPolicy := rh.cmd.Spec.ConcurrencyPolicy
	if concurrencyPolicy == "" {
		concurrencyPolicy = batchv1beta1.ForbidConcurrent
	}

	job := rh.newJob()

	return batchv1beta1.CronJobSpec{
		JobTemplate: batchv1beta1.JobTemplateSpec{
			ObjectMeta: job.ObjectMeta,
			Spec:       job.Spec,
		},
		Schedule:                   rh.cmd.Spec.Schedule,
		Suspend:                    &suspend,
		StartingDeadlineSeconds:    &startingDeadlineSeconds,
		ConcurrencyPolicy:          concurrencyPolicy,
		SuccessfulJobsHistoryLimit: &historyLimit,
		FailedJobsHistoryLimit:     &historyLimit,
	}
}

func jobName(cmd *fnv1alpha1.Command) string {
	return "command-" + cmd.Name
}
