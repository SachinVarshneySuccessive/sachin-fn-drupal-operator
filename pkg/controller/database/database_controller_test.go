package database

import (
	"context"
	"testing"
	"time"

	"github.com/acquia/fn-drupal-operator/pkg/testhelpers"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"
)

func TestDatabaseController_UnknownName(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	unknownName := "unknown"

	// objects to track in the fake client
	fakeObjects := []runtime.Object{}

	r := buildFakeReconcile(fakeObjects)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: unknownName,
		},
	}

	// should not execute the reconcile loop if Database with provided name doesn't exists.
	res, err := r.Reconcile(req)
	require.NoError(t, err)
	require.False(t, res.Requeue)
}

func testMigration(r *ReconcileDatabase, req reconcile.Request) func(t *testing.T) {
	return func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verify the migration
		database := &fnv1alpha1.Database{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, database)
		require.NoError(t, err)
		require.Equal(t, "2", database.GetLabels()[fnv1alpha1.VersionLabel])
	}
}

func testDatabaseLabel(r *ReconcileDatabase, req reconcile.Request) func(t *testing.T) {
	return func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		database := &fnv1alpha1.Database{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, database)
		require.NoError(t, err)
		require.True(t, testhelpers.IsValidUUID(database.Id()))
	}
}

func TestDatabaseController_KnownName(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	fakeObjects := []runtime.Object{testDatabase}

	r := buildFakeReconcile(fakeObjects)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testName,
			Namespace: testNameSpace,
		},
	}

	t.Run("should perform migration", testMigration(r, req))

	t.Run("should add a label to the database object", testDatabaseLabel(r, req))

	t.Run("should add the finalizers to database object", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// Verify the finalizers
		database := &fnv1alpha1.Database{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, database)
		require.NoError(t, err)
		require.True(t, common.HasFinalizer(database, dbPwdSecretFinalizer))
	})

	t.Run("should create the user secret", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		secret := &corev1.Secret{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: testDatabase.Spec.UserSecret, Namespace: req.Namespace}, secret)
		require.NoError(t, err)

		// Verify that password has been properly set
		require.NotEmpty(t, secret.Data["password"])
		// Verify that Owner References have been applied
		require.Equal(t, secret.OwnerReferences[0].Kind, "Database")
		require.Equal(t, secret.OwnerReferences[0].Name, testName)
	})

	t.Run("should not reconcile database and throw error", func(t *testing.T) {
		_, err := r.Reconcile(req)
		// expecting an error because of missing "admin secrets".
		require.EqualError(t, err, "secrets \"wlgore-admin-secret\" not found")
	})

	// Manually creating the admin secrets
	err := r.client.Create(context.TODO(), adminSecret)
	require.NoError(t, err)

	t.Run("should add a finalizer to the database admin secret", func(t *testing.T) {
		setAdminConnectionFunc(mockCreateDatabaseNoError)
		defer restoreAdminConnectionFunc()

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.False(t, res.Requeue)

		secret := &corev1.Secret{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: testDatabase.Spec.AdminSecret, Namespace: req.Namespace}, secret)
		require.NoError(t, err)

		// Verify that finalizers have been applied properly
		require.True(t, common.HasFinalizer(secret, dbPwdSecretFinalizer))
	})

	t.Run("should throw error while creating database", func(t *testing.T) {
		// Mocking of SQL queries
		setAdminConnectionFunc(mockCreateDatabseSQLError)
		defer restoreAdminConnectionFunc()

		res, err := r.Reconcile(req)
		require.EqualError(t, err, "all expectations were already fulfilled, call to database Close was not expected")
		require.False(t, res.Requeue)
	})

	t.Run("should throw error if not able to close sql connect", func(t *testing.T) {
		// Mocking of SQL queries
		setAdminConnectionFunc(mockCreateDatabaseAndUserWithoutClose)
		defer restoreAdminConnectionFunc()

		res, err := r.Reconcile(req)
		require.EqualError(t, err, "all expectations were already fulfilled, call to database Close was not expected")
		require.False(t, res.Requeue)
	})

	t.Run("should create the database", func(t *testing.T) {
		// Mocking of SQL queries
		setAdminConnectionFunc(mockCreateDatabaseAndUser)
		defer restoreAdminConnectionFunc()

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.False(t, res.Requeue)
	})
}

// func TestDatabaseController_KnownNameWithoutAdminSecret(t *testing.T) {
// 	// Set the logger to development mode for verbose logs.
// 	logf.SetLogger(logf.ZapLogger(true))

// 	// objects to track in the fake client
// 	fakeObjects := []runtime.Object{testDatabaseWithoutAdminSecret}

// 	r := buildFakeReconcile(fakeObjects)

// 	req := reconcile.Request{
// 		NamespacedName: types.NamespacedName{
// 			Name:      testName,
// 			Namespace: testNameSpace,
// 		},
// 	}

// 	t.Run("should perform migration", testMigration(r, req))

// 	t.Run("should add a label to the database object", testDatabaseLabel(r, req))

// 	t.Run("should add the finalizers to database object", func(t *testing.T) {
// 		res, err := r.Reconcile(req)
// 		require.NoError(t, err)
// 		require.True(t, res.Requeue)

// 		// Verify the finalizers
// 		database := &fnv1alpha1.Database{}
// 		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, database)
// 		require.NoError(t, err)
// 		require.True(t, common.HasFinalizer(database, dbPwdSecretFinalizer))
// 	})

// 	t.Run("should create the user secret", func(t *testing.T) {
// 		res, err := r.Reconcile(req)
// 		require.NoError(t, err)
// 		require.True(t, res.Requeue)

// 		secret := &corev1.Secret{}
// 		err = r.client.Get(context.TODO(), types.NamespacedName{Name: testDatabaseWithoutAdminSecret.Spec.UserSecret, Namespace: req.Namespace}, secret)
// 		require.NoError(t, err)

// 		// Verify that password has been properly set
// 		require.NotEmpty(t, secret.Data["password"])
// 		// Verify that Owner References have been applied
// 		require.Equal(t, secret.OwnerReferences[0].Kind, "Database")
// 		require.Equal(t, secret.OwnerReferences[0].Name, testName)
// 	})

// 	t.Run("should not reconcile database and throw error", func(t *testing.T) {
// 		_, err := r.Reconcile(req)
// 		require.NoError(t, err)
// 	})

// 	t.Run("should throw error while creating database", func(t *testing.T) {
// 		// Mocking of SQL queries
// 		setAdminConnectionFunc(mockCreateDatabseSQLError)
// 		defer restoreAdminConnectionFunc()

// 		res, err := r.Reconcile(req)
// 		require.NoError(t, err)
// 		// require.EqualError(t, err, "all expectations were already fulfilled, call to database Close was not expected")
// 		require.False(t, res.Requeue)
// 	})

// 	t.Run("should throw error if not able to close sql connect", func(t *testing.T) {
// 		// Mocking of SQL queries
// 		setAdminConnectionFunc(mockCreateDatabaseAndUserWithoutClose)
// 		defer restoreAdminConnectionFunc()

// 		res, err := r.Reconcile(req)
// 		require.NoError(t, err)
// 		// require.EqualError(t, err, "all expectations were already fulfilled, call to database Close was not expected")
// 		require.False(t, res.Requeue)
// 	})

// 	t.Run("should create the database", func(t *testing.T) {
// 		// Mocking of SQL queries
// 		setAdminConnectionFunc(mockCreateDatabaseAndUser)
// 		defer restoreAdminConnectionFunc()

// 		res, err := r.Reconcile(req)
// 		require.NoError(t, err)
// 		require.False(t, res.Requeue)
// 	})
// }
func TestDatabaseController_Deletion(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	fakeObjects := []runtime.Object{
		testDatabaseForDeletion,
		userSecret,
		adminSecretWithFinalizer,
	}

	r := buildFakeReconcile(fakeObjects)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testName,
			Namespace: testNameSpace,
		},
	}

	// Setting up the DeletionTime of database object to non-zero to simulate k8s delete event
	database := &fnv1alpha1.Database{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: testDatabase.Name, Namespace: req.Namespace}, database)
	require.NoError(t, err)

	database.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
	err = r.client.Update(context.TODO(), database)
	require.NoError(t, err)

	t.Run("should perform migration", testMigration(r, req))

	t.Run("should add a label to the database object", testDatabaseLabel(r, req))

	t.Run("should throw error during deletion of database", func(t *testing.T) {
		// Mocking of SQL queries
		setAdminConnectionFunc(mockRemoveDBNotExists)
		defer restoreAdminConnectionFunc()

		res, err := r.Reconcile(req)
		require.EqualError(t, err, "Error 1: Something went wrong")
		require.False(t, res.Requeue)
	})

	t.Run("should throw error during deletion of user", func(t *testing.T) {
		// Mocking of SQL queries
		setAdminConnectionFunc(mockRemoveDropUserError)
		defer restoreAdminConnectionFunc()

		res, err := r.Reconcile(req)
		require.EqualError(t, err, "Error 1: Something went wrong")
		require.False(t, res.Requeue)
	})

	// Mocking of SQL queries
	setAdminConnectionFunc(mockRemoveGetAdminDatabaseConnection)
	defer restoreAdminConnectionFunc()

	t.Run("should remove finalizer from the database admin secret", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		secret := &corev1.Secret{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: testDatabase.Spec.AdminSecret, Namespace: req.Namespace}, secret)
		require.NoError(t, err)
		// admin secret finalizers should have been removed
		require.False(t, common.HasFinalizer(secret, dbPwdSecretFinalizer))
	})

	t.Run("should remove the finalizers from database object", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.True(t, res.Requeue)

		// database finalizers should have been removed
		database := &fnv1alpha1.Database{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, database)
		require.NoError(t, err)
		require.False(t, common.HasFinalizer(database, dbPwdSecretFinalizer))
	})

	t.Run("finishing up of job", func(t *testing.T) {
		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.False(t, res.Requeue)
	})

	t.Run("deletion of secret should not have any effect", func(t *testing.T) {
		secret := &corev1.Secret{}

		err = r.client.Get(context.TODO(), types.NamespacedName{Name: testDatabase.Spec.UserSecret, Namespace: req.Namespace}, secret)
		require.NoError(t, err)
		err = r.client.Delete(context.TODO(), secret)
		require.NoError(t, err)

		res, err := r.Reconcile(req)
		require.NoError(t, err)
		require.False(t, res.Requeue)
	})
}

func TestDatabaseController_DeletionNoAdminSecret(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	// objects to track in the fake client
	fakeObjects := []runtime.Object{
		testDatabaseForDeletion,
		userSecret,
	}

	r := buildFakeReconcile(fakeObjects)
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      testName,
			Namespace: testNameSpace,
		},
	}

	// Setting up the DeletionTime of database object to non-zero to simulate k8s delete event
	database := &fnv1alpha1.Database{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: testDatabase.Name, Namespace: req.Namespace}, database)
	require.NoError(t, err)

	database.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
	err = r.client.Update(context.TODO(), database)
	require.NoError(t, err)

	t.Run("should perform migration", testMigration(r, req))

	t.Run("should add a label to the database object", testDatabaseLabel(r, req))

	t.Run("should not throw an error if admin secret doesn't exist", func(t *testing.T) {
		res, err := r.Reconcile(req)

		// Finalize admin secret should not throw an error if secret is already removed.
		database := &fnv1alpha1.Database{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, database)
		require.NoError(t, err)
		require.False(t, common.HasFinalizer(database, dbPwdSecretFinalizer))

		res, err = r.Reconcile(req)
		require.NoError(t, err)
		require.False(t, res.Requeue)
	})
}
