package drupalenvironment

import (
	"context"
	"fmt"
	"time"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"
)

const (
	drenvCleanupFinalizer = "drupalenvironments.fnresources.acquia.io"
	istioInjectionLabel   = "istio-injection"
)

var log = logf.Log.WithName("controller_drupalenvironment")

// Below methods are generated boilerplate and won't need to be unit tested

// Add creates a new DrupalEnvironment Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDrupalEnvironment{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("drupalenvironment-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 30})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource DrupalEnvironment
	err = c.Watch(&source.Kind{Type: &fnv1alpha1.DrupalEnvironment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner DrupalEnvironment
	err = common.WatchOwned(c, &fnv1alpha1.DrupalEnvironment{}, []runtime.Object{
		&v1.ConfigMap{},
		// &v1.PersistentVolumeClaim{}, // Probably doesn't need to be watched, as its spec can't be changed
		&v1.Secret{},
		&v1.Service{},
		&v1.ServiceAccount{},
		&appsv1.Deployment{},
		&appsv1.StatefulSet{},
		&autoscalingv1.HorizontalPodAutoscaler{},
		&rolloutsv1alpha1.Rollout{},
	})
	return err
}

var _ reconcile.Reconciler = &ReconcileDrupalEnvironment{}

// ReconcileDrupalEnvironment reconciles a DrupalEnvironment object
type ReconcileDrupalEnvironment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a DrupalEnvironment object and makes changes based on the state read
// and what is in the DrupalEnvironment.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDrupalEnvironment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace)
	fmt.Print("\n\n  This is sachin again \n\n  ============================================> \n\n ")

	// Fetch the DrupalEnvironment instance
	env := &fnv1alpha1.DrupalEnvironment{}
	err := r.client.Get(context.TODO(), request.NamespacedName, env)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Error reading the object - requeue the request.
			logger.Error(err, "Failed to get DrupalEnvironment")
			return reconcile.Result{}, err
		}
		// Request object not found. Most likely it's been deleted, so do nothing
		return reconcile.Result{}, nil
	}

	// migrate to new Spec FIRST before anything else so we know for the rest
	// of reconciliation that the type we have is safe
	currentVersion := fnv1alpha1.ObjectVersion(env)
	if fnv1alpha1.Migrate(env) {
		logger.Info("MIGRATING", "current version", currentVersion, "target version", env.SpecVersion())
		return reconcile.Result{Requeue: true}, r.client.Update(context.TODO(), env)
	}

	if env.Id() == "" {
		env.SetId(uuid.New().String())
		if err := r.client.Update(context.TODO(), env); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Create a requestHandler instance that will service this reconcile request
	rh := &requestHandler{
		reconciler: r,
		namespace:  request.Namespace,
		env:        env,
		app:        &fnv1alpha1.DrupalApplication{},
		logger:     logger,
	}

	result, err := rh.doReconcile()

	// Updating the Environment Status
	statusError := rh.updateEnvironmentStatus(result, err)
	if err == nil && statusError != nil {
		err = statusError
		rh.logger.Error(statusError, "Failed to update environment status")
	}

	return result, err
}

func (rh *requestHandler) isMarkedForDeletion() bool {
	return rh.env.GetDeletionTimestamp() != nil
}

func (rh *requestHandler) doReconcile() (result reconcile.Result, err error) {
	r := rh.reconciler

	// Check if this resource is being deleted
	if rh.isMarkedForDeletion() {
		// Clean up non-owned Resources
		if !common.UseDynamicProvisioning() {
			result.Requeue, err = rh.finalizePV()
			if common.ShouldReturn(result, err) {
				return
			}
		}

		result.Requeue, err = rh.finalizeSSHDAccessControls()
		if common.ShouldReturn(result, err) {
			return
		}

		// Remove our finalizer
		if common.HasFinalizer(rh.env, drenvCleanupFinalizer) {
			rh.logger.Info("Removing finalizer")
			controllerutil.RemoveFinalizer(rh.env, drenvCleanupFinalizer)

			err = r.client.Update(context.TODO(), rh.env)
			result.Requeue = true
			return
		}

		return
	}

	// Ensure our DrupalEnvironment has a finalizer, for cleaning up the PV
	if !common.HasFinalizer(rh.env, drenvCleanupFinalizer) {
		rh.logger.Info("Adding finalizer")
		controllerutil.AddFinalizer(rh.env, drenvCleanupFinalizer)
		if err = r.client.Update(context.TODO(), rh.env); err != nil {
			rh.logger.Error(err, "Failed to update controller reference")
			return
		}
		result.Requeue = true
		return
	}

	// Label the DrupalEnvironment with the SHA1 hash of its "gitRef" field
	hashedGitRef := common.HashValueForLabel(rh.env.Spec.GitRef)

	if rh.env.Labels[fnv1alpha1.GitRefLabel] != hashedGitRef {
		// Update the labels
		rh.env.Labels[fnv1alpha1.GitRefLabel] = hashedGitRef
		err := r.client.Update(context.TODO(), rh.env)
		if err != nil {
			rh.logger.Error(err, "Failed to set git ref label", "Name", rh.env.Name)
			return reconcile.Result{}, err
		}
		rh.logger.Info("Git ref label updated")
		return reconcile.Result{Requeue: true}, nil
	}

	if requeue, err := rh.labelNamespaceForIstio(); requeue || err != nil {
		return reconcile.Result{Requeue: requeue}, err
	}

	// Fetch the parent DrupalApplication instance
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: rh.env.Spec.Application}, rh.app)
	if err != nil {
		if errors.IsNotFound(err) {
			rh.logger.Info("Parent Application doesn't exist", "Application Name", rh.app.Name)
			// Delay the requeue rather than returning an error, to avoid exponential error backoff
			return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
		} else {
			rh.logger.Error(err, "Failed to get Application", "Application Name", rh.app.Name)
			return reconcile.Result{}, err
		}
	}

	// Reconcile owner reference and sync labels from owner
	update, err := common.LinkToOwner(rh.app, rh.env, r.scheme)
	if err != nil {
		return reconcile.Result{}, err
	}
	if update {
		if err := r.client.Update(context.TODO(), rh.env); err != nil {
			rh.logger.Error(err, "Failed to update controller reference")
			return reconcile.Result{}, err
		} else {
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// PHP settings ConfigMap
	phpConfig := make(map[string]string)
	phpConfig["zzz_drupalenvironment.ini"] = fmt.Sprintf(`
max_input_vars = %v
max_execution_time = %v
memory_limit = %vM
post_max_size = %vM
apc.shm_size = %vM
opcache.memory_consumption = %v
opcache.interned_strings_buffer = %v
session.save_path = "/shared/php_sessions"
`,
		rh.env.Spec.Phpfpm.MaxInputVars,
		rh.env.Spec.Phpfpm.MaxExecutionTime,
		rh.env.Spec.Phpfpm.ProcMemoryLimitMiB,
		rh.env.Spec.Phpfpm.PostMaxSizeMiB,
		rh.env.Spec.Phpfpm.ApcMemoryLimitMiB,
		rh.env.Spec.Phpfpm.OpcacheMemoryLimitMiB,
		rh.env.Spec.Phpfpm.OpcacheInternedStringsBufferMiB)

	phpConfig["zzz_drupalenvironment_cli.ini"] = fmt.Sprintf(`
max_input_vars = %v
post_max_size = %vM
apc.shm_size = %vM
opcache.memory_consumption = %v
opcache.interned_strings_buffer = %v
session.save_path = "/shared/php_sessions"
`,
		rh.env.Spec.Phpfpm.MaxInputVars,
		rh.env.Spec.Phpfpm.PostMaxSizeMiB,
		rh.env.Spec.Phpfpm.ApcMemoryLimitMiB,
		rh.env.Spec.Phpfpm.OpcacheMemoryLimitMiB,
		rh.env.Spec.Phpfpm.OpcacheInternedStringsBufferMiB)

	// Configure New Relic if a license key was given
	if rh.env.Spec.Phpfpm.NewRelicSecret != "" {
		conf, err := rh.newRelicConf()
		if err == nil {
			phpConfig["newrelic.ini"] = conf
		} else {
			rh.logger.Error(err, "couldn't generate New Relic config file")
		}
	}

	var requeue bool
	requeue, err = rh.reconcileConfigMap("php-config", phpConfig)
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	// Create/Update phpfpm ConfigMap
	result, err = rh.reconcilePhpFpmConfigMap()
	if common.ShouldReturn(result, err) {
		return
	}

	// Create/Update apache-conf-enabled ConfigMap
	result, err = rh.reconcileApacheConfEnabledConfigMap()
	if common.ShouldReturn(result, err) {
		return
	}

	// Check if the PV and PVC already exist, if not create them
	if !common.UseDynamicProvisioning() {
		requeue, err = rh.reconcilePV()
		if err != nil || requeue {
			return reconcile.Result{Requeue: requeue}, err
		}
	}

	requeue, err = rh.reconcilePVC()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	requeue, err = rh.reconcileDrupalService()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	// Create/Update Environment Config Secret
	result, err = rh.reconcileEnvConfigSecret()
	if common.ShouldReturn(result, err) {
		return
	}

	requeue, err = rh.reconcileDrupalRollout()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	requeue, err = rh.reconcileHPA()
	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	// Reconcile SSHD resources
	sshUsername, err := rh.getSSHUsername()
	if err == nil {
		result, err := rh.reconcileSSHDAccessControls()
		if err != nil || result.Requeue || result.RequeueAfter != 0 {
			return result, err
		}

		result, err = rh.reconcileSSHDService(sshUsername)
		if err != nil || result.Requeue || result.RequeueAfter != 0 {
			return result, err
		}

		result, err = rh.reconcileSSHDDeployment(sshUsername)
		if err != nil || result.Requeue || result.RequeueAfter != 0 {
			return result, err
		}
	} else {
		// If an API error occurred, return it
		if err, ok := err.(*errors.StatusError); ok {
			rh.logger.Error(err, "Failed to get SSH username")
			return reconcile.Result{}, err
		}

		// Otherwise, a ConfigMap misconfiguration is in place, so just log the error
		rh.logger.Info("Couldn't configure SSH Endpoint", "reason", err)
	}

	return reconcile.Result{}, nil
}

type requestHandler struct {
	reconciler *ReconcileDrupalEnvironment

	env       *fnv1alpha1.DrupalEnvironment
	app       *fnv1alpha1.DrupalApplication
	namespace string
	logger    logr.Logger
}

func (rh *requestHandler) associateResourceWithController(o metav1.Object) {
	err := controllerutil.SetControllerReference(rh.env, o, rh.reconciler.scheme)
	if err != nil {
		rh.logger.Error(err, "Failed to set controller as owner", "Resource", o)
	}
}

// Add label to namespace if istio is enabled, to trigger auto-injection of Envoy
// Remove the label if istio is disabled
func (rh *requestHandler) labelNamespaceForIstio() (requeue bool, err error) {
	r := rh.reconciler
	ns := &v1.Namespace{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: rh.namespace}, ns)
	if err != nil {
		return
	}

	_, nsLabeled := ns.Labels[istioInjectionLabel]
	if common.IsIstioEnabled() && !nsLabeled {
		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		ns.Labels[istioInjectionLabel] = "enabled"
		rh.logger.Info("Adding istio label to namespace.")
		requeue = true
		err = r.client.Update(context.TODO(), ns)
	} else if !common.IsIstioEnabled() && nsLabeled {
		delete(ns.Labels, istioInjectionLabel)
		rh.logger.Info("Removing istio label from namespace.")
		requeue = true
		err = r.client.Update(context.TODO(), ns)
	}

	return
}

func (rh *requestHandler) getRunningPodsCount(labels map[string]string) (count int32, err error) {
	podsList := &v1.PodList{}
	err = rh.reconciler.client.List(context.TODO(), podsList, client.MatchingLabels(labels))
	count = 0
	for _, pod := range podsList.Items {
		if pod.Status.Phase == v1.PodRunning {
			count++
		}
	}
	return count, nil
}

func (rh *requestHandler) getEnvironmentStatus(result reconcile.Result, recError error) (status fnv1alpha1.DrupalEnvironmentStatusType) {
	if recError != nil {
		return fnv1alpha1.DrupalEnvironmentStatusUnstable
	}

	if rh.isMarkedForDeletion() {
		return fnv1alpha1.DrupalEnvironmentStatusDeleting
	}

	if result.Requeue || result.RequeueAfter > 0 {
		return fnv1alpha1.DrupalEnvironmentStatusSyncing
	}

	r := rh.reconciler
	rollout := &rolloutsv1alpha1.Rollout{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: drupalRolloutName, Namespace: rh.namespace}, rollout)

	if err != nil {
		if errors.IsNotFound(err) {
			return fnv1alpha1.DrupalEnvironmentStatusSyncing
		}
		return fnv1alpha1.DrupalEnvironmentStatusUnstable
	}

	if isNewRSAvailable(rollout) && isAvailable(rollout) {
		return fnv1alpha1.DrupalEnvironmentStatusSynced
	}

	if isReplicaSetUpdated(rollout) {
		return fnv1alpha1.DrupalEnvironmentStatusDeploying
	}

	return fnv1alpha1.DrupalEnvironmentStatusDeployError
}

func (rh *requestHandler) updateEnvironmentStatus(result reconcile.Result, recError error) (err error) {
	r := rh.reconciler

	drupalLabels := labelsForRollout(rh.env)
	drupalCount, err := rh.getRunningPodsCount(drupalLabels)
	if err != nil {
		return err
	}

	status := rh.getEnvironmentStatus(result, recError)

	nextStatus := fnv1alpha1.DrupalEnvironmentStatus{
		NumDrupal: drupalCount,
		Status:    status,
	}

	// Retrieving the actual DrupalEnvironment's runtime object for the status comparison & whether there is a need for update.
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: rh.env.Name, Namespace: rh.namespace}, rh.env)
	if err != nil {
		return err
	}

	if !cmp.Equal(nextStatus, rh.env.Status) {
		rh.env.Status = nextStatus
		err = r.client.Status().Update(context.TODO(), rh.env)
		if err != nil {
			return err
		}
	}

	return nil
}

func (rh *requestHandler) reconcileConfigMap(name string, data map[string]string) (requeue bool, err error) {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
		},
	}
	logger := rh.logger.WithValues("Namespace", cm.Namespace, "Name", cm.Name)

	op, err := controllerutil.CreateOrUpdate(context.TODO(), rh.reconciler.client, cm, func() error {
		if cm.CreationTimestamp.IsZero() {
			cm.Labels = common.MergeLabels(cm.Labels, rh.env.ChildLabels())
			rh.associateResourceWithController(cm)
		}
		cm.Data = data
		return nil
	})
	if err != nil {
		logger.Error(err, "Failed to reconcile ConfigMap")
		return false, err
	}
	if op != controllerutil.OperationResultNone {
		logger.Info("Reconciled ConfigMap", "operation", op)
		return true, nil
	}
	return
}
