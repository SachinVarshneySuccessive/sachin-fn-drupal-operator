package site

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extv1b1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
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

	fn "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"
)

// siteCleanupFinalizer defines the site finalizer.
const siteCleanupFinalizer = "sites.fnresources.acquia.io"

// dbPwdSecretFinalizer defines the database password secret finalizer.
const dbPwdSecretFinalizer = "sites.fnresources.acquia.com/password"

var log = logf.Log.WithName("controller_site")

// Below methods are generated boilerplate and won't need to be unit tested

// Add creates a new Site Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSite{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("site-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 30})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Site
	// err = c.Watch(&source.Kind{Type: &fn.Site{}}, &handler.EnqueueRequestsFromMapFunc{})
	if err := c.Watch(&source.Kind{Type: &fn.Site{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for secondary resources created by and owned exclusively by a Site
	err = common.WatchOwned(c, &fn.Site{}, []runtime.Object{
		&corev1.Secret{},
		&extv1b1.Ingress{},
		&batchv1b1.CronJob{}, // FIXME?
	})
	return err
}

// blank assignment to verify that ReconcileSite implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSite{}

// ReconcileSite reconciles a Site object
type ReconcileSite struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// requestHandler gets initialized per request to have thread-safe code.
type requestHandler struct {
	reconciler *ReconcileSite

	app      *fn.DrupalApplication
	env      *fn.DrupalEnvironment
	site     *fn.Site
	database *fn.Database
	logger   logr.Logger
}

// Reconcile reads that state of the cluster for a Site object and makes changes based on the state read
// and what is in the Site.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSite) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Create a requestHandler instance that will service this reconcile request
	rh := requestHandler{
		reconciler: r,
		app:        &fn.DrupalApplication{},
		env:        &fn.DrupalEnvironment{},
		site:       &fn.Site{},
		database:   &fn.Database{},
		logger:     log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace),
	}

	rh.logger.Info("Reconciling site", "Request", request)
	// Fetch the Site instance
	err := rh.reconciler.client.Get(context.TODO(), request.NamespacedName, rh.site)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Error reading the object - requeue the request.
			return reconcile.Result{}, err
		}
		// Request object not found. Most likely it's been deleted, so do nothing
		return reconcile.Result{}, nil
	}

	if rh.site.Id() == "" {
		rh.site.SetId(uuid.New().String())
		if err := r.client.Update(context.TODO(), rh.site); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	if reqResult, err := rh.doReconcile(); reqResult.Requeue || reqResult.RequeueAfter > 0 || err != nil {
		return reqResult, err
	}

	// Note: the status of the site gets updated during the reconcile process, at this point the status needs to be updated.
	err = rh.reconciler.client.Status().Update(context.TODO(), rh.site)
	if err != nil {
		log.Error(err, "Unable to update site status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// doReconcile runs the reconcile loop.
func (rh *requestHandler) doReconcile() (result reconcile.Result, err error) {
	rh.site.SetStatus(fn.SiteSyncingStatus)

	isSiteMarkedToBeDeleted := rh.site.GetDeletionTimestamp() != nil
	if isSiteMarkedToBeDeleted {
		rh.setSiteFinalizingStatuses()

		// Remove this site's entry from the env-config Secret
		result, err = rh.cleanupSiteSettings()
		if common.ShouldReturn(result, err) {
			return
		}

		if requeue, err := rh.removeFinalizer(); requeue || err != nil {
			return reconcile.Result{Requeue: requeue}, err
		}
	}

	if requeue, err := rh.addFinalizer(); requeue || err != nil {
		return reconcile.Result{Requeue: requeue}, err
	}

	// Get parent environment
	err = rh.reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: rh.site.Namespace, Name: rh.site.Spec.Environment}, rh.env)
	if err != nil {
		if errors.IsNotFound(err) {
			// Delay the requeue rather than returning an error, to avoid exponential error backoff
			rh.logger.Info("Failed to get parent environment", "Environment", rh.site.Spec.Environment)
			result.RequeueAfter = time.Second * 10
			return result, nil
		}
		return
	}

	// Get super-parent application
	err = rh.reconciler.client.Get(context.TODO(), types.NamespacedName{Name: rh.env.Spec.Application}, rh.app)
	if err != nil {
		if errors.IsNotFound(err) {
			rh.logger.Info("Failed to get parent application", "Application", rh.env.Spec.Application)
			result.RequeueAfter = time.Second * 10
			return result, nil
		}
		return
	}

	// Get database
	err = rh.reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: rh.site.Namespace, Name: rh.site.Spec.Database}, rh.database)
	if err != nil {
		if errors.IsNotFound(err) {
			rh.logger.Info("Failed to get database", "Database", rh.site.Spec.Database)
			result.RequeueAfter = time.Second * 10
			return result, nil
		}
		return
	}

	result.Requeue, err = rh.linkToEnvironment()
	if err != nil {
		rh.logger.Error(err, "Failed to link to parent Environment")
	}
	if common.ShouldReturn(result, err) {
		return
	}

	// Reconcile Ingress and Site Settings
	result, err = rh.reconcileDomains()
	if common.ShouldReturn(result, err) {
		return
	}

	rh.site.SetStatus(fn.SiteSyncedStatus)
	return
}

// addFinalizer adds a finalizer to the Site CR, to coordinate cleanup of Site Settings
func (rh *requestHandler) addFinalizer() (requeue bool, err error) {
	if !common.HasFinalizer(rh.site, siteCleanupFinalizer) {
		controllerutil.AddFinalizer(rh.site, siteCleanupFinalizer)
		return true, rh.reconciler.client.Update(context.TODO(), rh.site)
	}
	return false, nil
}

func (rh *requestHandler) removeFinalizer() (requeue bool, err error) {
	if common.HasFinalizer(rh.site, siteCleanupFinalizer) {
		controllerutil.RemoveFinalizer(rh.site, siteCleanupFinalizer)
		return true, rh.reconciler.client.Update(context.TODO(), rh.site)
	}
	return false, nil
}

// linkToEnvironment sets the owner reference of the Site to the DrupalEnvironment it is hosted in, and
// updates Site labels with those from the environment.
func (rh *requestHandler) linkToEnvironment() (requeue bool, err error) {
	update, err := common.LinkToOwner(rh.env, rh.site, rh.reconciler.scheme)
	if err != nil {
		return false, err
	}
	if update {
		if err := rh.reconciler.client.Update(context.TODO(), rh.site); err != nil {
			return false, err
		}
	}

	return update, nil
}

// reconcileDomains creates the domains config maps and ingress objects.
func (rh *requestHandler) reconcileDomains() (result reconcile.Result, err error) {
	rh.site.SetDomainStatus(fn.DomainsUpdatingStatus)

	result, err = rh.updateSiteSettings()
	if common.ShouldReturn(result, err) {
		return
	}

	result.Requeue, err = rh.reconcileIngress()
	if common.ShouldReturn(result, err) {
		return
	}

	rh.site.SetDomainStatus(fn.DomainsSyncedStatus)
	return
}

// setSiteFinalizingStatuses sets the site status and sun statuses to an empty string.
func (rh *requestHandler) setSiteFinalizingStatuses() {
	rh.site.SetDomainStatus(fn.DomainsCleanUpStatus)
	rh.site.SetStatus(fn.SiteCleanUpStatus)
}
