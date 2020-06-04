package database

import (
	"context"
	"database/sql"
	goErr "errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
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

// dbPwdSecretFinalizer defines the database password secret finalizer.
const dbPwdSecretFinalizer = "database.fnresources.acquia.com/password"

var log = logf.Log.WithName("controller_database")

// Add creates a new Database Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// Also registers webhooks for this type.
func Add(mgr manager.Manager) error {
	err := builder.
		WebhookManagedBy(mgr).
		For(&fn.Database{}).
		Complete()
	if err != nil {
		log.Error(err, "could not create database webhook")
		return err
	}
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDatabase{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("database-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Database
	err = c.Watch(&source.Kind{Type: &fn.Database{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource secrets and requeue the owner Database
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &fn.Database{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileDatabase implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileDatabase{}

// ReconcileDatabase reconciles a Database object
type ReconcileDatabase struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// requestHandler gets initialized per request to have thread-safe code.
type requestHandler struct {
	reconciler *ReconcileDatabase
	namespace  string
	database   *fn.Database
	logger     logr.Logger
}

// Useful for mocking the MYSQL datbase
var getAdminDatabaseConnection = func(database *fn.Database, client client.Client) (*sql.DB, error) {
	return database.GetAdminConnection(client)
}

// Reconcile starts the reconcile loop
func (r *ReconcileDatabase) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Create a requestHandler instance that will service this reconcile request
	rh := requestHandler{
		reconciler: r,
		database:   &fn.Database{},
		namespace:  request.Namespace,
		logger:     log.WithValues("Request.Name", request.Name, "Request.Namespace", request.Namespace),
	}

	// Get database
	rh.logger.V(1).Info("Reconciling Database")
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: request.Name, Namespace: rh.namespace}, rh.database)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// migrate to new Spec FIRST before anything else so we know for the rest
	// of reconciliation that the type we have is safe
	currentVersion := fn.ObjectVersion(rh.database)
	if fn.Migrate(rh.database) {
		rh.logger.Info("MIGRATING", "current version", currentVersion, "target version", rh.database.SpecVersion())
		return reconcile.Result{Requeue: true}, r.client.Update(context.TODO(), rh.database)
	}

	if rh.database.Id() == "" {
		rh.database.SetId(uuid.New().String())
		if err := r.client.Update(context.TODO(), rh.database); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	isDatabaseMarkedToBeDeleted := rh.database.GetDeletionTimestamp() != nil
	if isDatabaseMarkedToBeDeleted {
		if err := rh.finalizeDatabase(); err != nil {
			return reconcile.Result{}, err
		}

		if requeue, err := rh.removeDbAdminFinalizer(); requeue || err != nil {
			return reconcile.Result{Requeue: requeue}, err
		}

		if requeue, err := rh.removeFinalizer(); requeue || err != nil {
			return reconcile.Result{Requeue: requeue}, err
		}

		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR.
	if requeue, err := rh.addFinalizer(); requeue || err != nil {
		return reconcile.Result{Requeue: requeue}, err
	}

	if requeue, err := rh.reconcileUserSecret(); requeue || err != nil {
		return reconcile.Result{Requeue: requeue}, err
	}

	if requeue, err := rh.reconcileDatabase(); requeue || err != nil {
		if err != nil {
			rh.logger.Error(err, "Failed to reconcile Database")
			return reconcile.Result{}, err
		}
		return reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}

	return reconcile.Result{}, nil
}

// associateResourceWithController sets subresource ownership
func (r *ReconcileDatabase) associateResourceWithController(o metav1.Object, d *fn.Database) {
	reqLogger := log
	err := controllerutil.SetControllerReference(d, o, r.scheme)
	if err != nil {
		reqLogger.Error(err, "Failed to set controller as owner", "Resource", o)
	}
}

// reconcileDatabase creates database and user based on database CR
func (rh *requestHandler) reconcileDatabase() (requeue bool, err error) {
	r := *rh.reconciler
	adminSecret := rh.database.Spec.AdminSecret
	userSecret := rh.database.Spec.UserSecret

	if adminSecret == "" {
		if userSecret == "" {
			rh.logger.Error(goErr.New("User secret is not provided"), "User secret is not provided")
			return false, goErr.New("User secret is not provided")
		}

		// pwd, err := rh.database.GetPassword(r.client)
		// if pwd == "" {
		// 	rh.logger.Error(err, "secret password is not provided")
		// 	return false, err
		// }

		return true, nil
	}

	dbName := rh.database.DatabaseName()
	dbUser := rh.database.Spec.User

	pwd, err := rh.database.GetPassword(r.client)
	if err != nil {
		return false, err
	}

	adminDB, err := getAdminDatabaseConnection(rh.database, r.client)
	if err != nil {
		return false, err
	}

	defer func() {
		err = adminDB.Close()
		if err != nil {
			rh.logger.Error(err, "adminDB.Close() failed")
		}
	}()

	if err := adminDB.Ping(); err != nil {
		rh.logger.Error(err, "adminDB.Ping() failed ")
		return true, nil
	}

	if requeue, err := rh.addDbAdminFinalizer(); requeue || err != nil {
		return requeue, err
	}

	_, err = adminDB.Exec("CREATE DATABASE IF NOT EXISTS `" + dbName + "`")
	if err != nil {
		rh.logger.Error(err, "Create database failed")
		return false, err
	}

	if _, err := adminDB.Exec(fmt.Sprintf("CREATE USER '%s'@'%%'", dbUser)); err != nil {
		// 1396 is ERR_CANNOT_USER in mysql5.6. In this case, it means the user already
		// exists in the system and cannot be created again.  This is the only error we
		// are happy to see, so we just log that there is nothing to do and move on.  All
		// other errors are failure cases.
		if driverError, ok := err.(*mysql.MySQLError); !ok || driverError.Number != 1396 {
			return false, err
		}
		rh.logger.Info("Can't create user, it already exists", "User", dbUser)
	}

	if _, err := adminDB.Exec(fmt.Sprintf("SET PASSWORD FOR '%s'@'%%' = PASSWORD('%s')", dbUser, pwd)); err != nil {
		return false, err
	}

	_, err = adminDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'", dbName, dbUser))
	if err != nil {
		return false, err
	}

	_, err = adminDB.Exec(fmt.Sprintf("FLUSH PRIVILEGES"))
	if err != nil {
		return false, err
	}

	rh.logger.V(1).Info("MySQL db/user reconciled", "Database", dbName, "User", dbUser)

	return false, nil
}

// reconcileUserSecret creates user-password secret for each database object
func (rh *requestHandler) reconcileUserSecret() (requeue bool, err error) {
	userSecret := &corev1.Secret{}

	err = rh.reconciler.client.Get(context.TODO(),
		types.NamespacedName{Namespace: rh.database.Namespace, Name: rh.database.Spec.UserSecret}, userSecret)
	if err != nil && errors.IsNotFound(err) {
		// generate password.
		rh.logger.Info("User Secret not found - creating", "Secret Name", rh.database.Spec.UserSecret)
		password, err := common.RandPassword()
		if err != nil {
			return false, err
		}

		userSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rh.database.Spec.UserSecret,
				Namespace: rh.database.Namespace,
			},
			// Defaulting sql user to databasename-user
			Data: map[string][]byte{
				"password": []byte(password),
			},
			Type: "Opaque",
		}
		// Associating secret with database controller
		rh.reconciler.associateResourceWithController(userSecret, rh.database)

		err = rh.reconciler.client.Create(context.TODO(), userSecret)
		return true, err

	} else if err != nil {
		return false, err
	}

	return false, err
}

// removeDbAdminFinalizer removes the database admin secret finalizer.
func (rh *requestHandler) removeDbAdminFinalizer() (requeue bool, err error) {
	dbSecret, err := rh.database.GetAdminSecret(rh.reconciler.client)
	if err != nil && errors.IsNotFound(err) {
		rh.logger.Info("Database admin Secret not found", "Secret Name", rh.database.Spec.AdminSecret)
		return false, nil
	} else if err != nil {
		return false, err
	}

	if common.HasFinalizer(dbSecret, dbPwdSecretFinalizer) {
		rh.logger.Info("Removing the database admin secret finalizer")
		controllerutil.RemoveFinalizer(dbSecret, dbPwdSecretFinalizer)
		if err := rh.reconciler.client.Update(context.TODO(), dbSecret); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil // Finalizer not found, already removed
}

// addFinalizer adds a finalizer to the Database CR, to coordinate cleanup of subresources
func (rh *requestHandler) addFinalizer() (requeue bool, err error) {
	if !common.HasFinalizer(rh.database, dbPwdSecretFinalizer) {
		controllerutil.AddFinalizer(rh.database, dbPwdSecretFinalizer)
		return true, rh.reconciler.client.Update(context.TODO(), rh.database)
	}
	return false, nil
}

func (rh *requestHandler) addDbAdminFinalizer() (requeue bool, err error) {
	dbSecret, err := rh.database.GetAdminSecret(rh.reconciler.client)
	if err != nil {
		return false, err
	}
	if !common.HasFinalizer(dbSecret, dbPwdSecretFinalizer) {
		controllerutil.AddFinalizer(dbSecret, dbPwdSecretFinalizer)
		return true, rh.reconciler.client.Update(context.TODO(), dbSecret)
	}
	return false, nil
}

func (rh *requestHandler) removeFinalizer() (requeue bool, err error) {
	if common.HasFinalizer(rh.database, dbPwdSecretFinalizer) {
		controllerutil.RemoveFinalizer(rh.database, dbPwdSecretFinalizer)
		return true, rh.reconciler.client.Update(context.TODO(), rh.database)
	}
	return false, nil
}

func (rh *requestHandler) finalizeDatabase() error {
	db := rh.database
	adminDB, err := getAdminDatabaseConnection(rh.database, rh.reconciler.client)
	if err != nil && errors.IsNotFound(err) {
		// If the admin secret has been already deleted we can't do any more database clean up.
		return nil
	} else if err != nil {
		return err
	}

	defer func() {
		err = adminDB.Close()
		if err != nil {
			rh.logger.Error(err, "adminDB.Close() failed")
		}
	}()

	if err := adminDB.Ping(); err != nil {
		return err
	}

	// Cleanup admin DB
	_, err = adminDB.Exec("DROP DATABASE IF EXISTS `" + db.DatabaseName() + "`")
	if err != nil {
		return err
	}

	if _, err = adminDB.Exec(fmt.Sprintf("DROP USER '%s'@'%%'", db.Spec.User)); err != nil {
		if driverError, ok := err.(*mysql.MySQLError); !ok || driverError.Number != 1396 {
			return err
		}
		rh.logger.Info("Cannot drop user, user not found", "User", db.Spec.User)
	}

	return nil
}
