package database

import (
	"database/sql"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/testhelpers"
)

var originalAdminConnectionFunc func(*fnv1alpha1.Database, client.Client) (*sql.DB, error)

const (
	testHost      = "mysql.default.svc.cluster.local"
	testPort      = 3306
	testName      = "wlgoredatabase"
	testNameSpace = "wlgore"
	testUser      = "wlgore"
	// testAdminSecret = "wlgore-admin-secret"
	testAdminSecret = ""
	testUserSecret  = "wlgore-user-secret"

	testAdminUsername = "admin"
	testAdminPassword = "adminpass"
)

var (
	testDatabase = &fnv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNameSpace,
		},
		Spec: fnv1alpha1.DatabaseSpec{
			Host:        testHost,
			Port:        testPort,
			SchemaName:  testName,
			User:        testUser,
			AdminSecret: testAdminSecret,
			UserSecret:  testUserSecret,
		},
	}
	testDatabaseWithoutAdminSecret = &fnv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNameSpace,
		},
		Spec: fnv1alpha1.DatabaseSpec{
			Host:       testHost,
			Port:       testPort,
			SchemaName: testName,
			User:       testUser,
			UserSecret: testUserSecret,
		},
	}
	testDatabaseForDeletion = &fnv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testName,
			Namespace:  testNameSpace,
			Finalizers: []string{dbPwdSecretFinalizer},
		},
		Spec: fnv1alpha1.DatabaseSpec{
			Host:        testHost,
			Port:        testPort,
			SchemaName:  testName,
			User:        testUser,
			AdminSecret: testAdminSecret,
			UserSecret:  testUserSecret,
		},
	}
	adminSecretWithFinalizer = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testAdminSecret,
			Namespace:  testNameSpace,
			Finalizers: []string{dbPwdSecretFinalizer},
		},
		StringData: map[string]string{
			"username": testAdminUsername,
			"password": testAdminPassword,
		},
		Type: "Opaque",
	}
	adminSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAdminSecret,
			Namespace: testNameSpace,
		},
		StringData: map[string]string{
			"username": testAdminUsername,
			"password": testAdminPassword,
		},
		Type: "Opaque",
	}
	userSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testUserSecret,
			Namespace: testNameSpace,
		},
		Data: map[string][]byte{
			"password": []byte("dummypassword"),
		},
		Type: "Opaque",
	}
)

func setAdminConnectionFunc(mockFunc func(*fnv1alpha1.Database, client.Client) (*sql.DB, error)) {
	originalAdminConnectionFunc = getAdminDatabaseConnection
	getAdminDatabaseConnection = mockFunc
}

func restoreAdminConnectionFunc() {
	getAdminDatabaseConnection = originalAdminConnectionFunc
}

// buildFakeReconcile return reconcile with fake client, schemes and runtime objects
func buildFakeReconcile(objects []runtime.Object) *ReconcileDatabase {
	client := testhelpers.NewFakeClient(objects)

	// create a ReconcileDatabase object with the scheme and fake client
	return &ReconcileDatabase{
		client: client,
		scheme: scheme.Scheme,
	}
}

func mockCreateDatabaseNoError(database *fnv1alpha1.Database, client client.Client) (*sql.DB, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, err
	}
	mock.ExpectClose()
	return db, err
}

func mockCreateDatabseSQLError(database *fnv1alpha1.Database, client client.Client) (*sql.DB, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, err
	}

	error := mysql.MySQLError{Number: 1396, Message: "User already exists"}
	mock.ExpectExec("CREATE DATABASE IF NOT EXISTS *").
		WillReturnError(&error)

	return db, err
}

func setCreateDBWithUserMock(mock sqlmock.Sqlmock) {
	mock.ExpectExec("CREATE DATABASE IF NOT EXISTS *").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mysql error code 1396 if user already exists
	error := mysql.MySQLError{Number: 1396, Message: "User already exists"}
	mock.ExpectExec("CREATE USER *@*").
		WillReturnError(&error)

	mock.ExpectExec("SET PASSWORD FOR *@*").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("GRANT ALL PRIVILEGES ON *").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("FLUSH PRIVILEGES").
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func mockCreateDatabaseAndUserWithoutClose(database *fnv1alpha1.Database, client client.Client) (*sql.DB, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, err
	}
	setCreateDBWithUserMock(mock)
	return db, err
}

func mockCreateDatabaseAndUser(database *fnv1alpha1.Database, client client.Client) (*sql.DB, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, err
	}
	setCreateDBWithUserMock(mock)
	mock.ExpectClose()
	return db, err
}

func mockRemoveDBNotExists(database *fnv1alpha1.Database, client client.Client) (*sql.DB, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, err
	}

	error := mysql.MySQLError{Number: 1, Message: "Something went wrong"}
	mock.ExpectExec("DROP DATABASE IF EXISTS*").
		WillReturnError(&error)

	mock.ExpectClose()
	return db, err
}

func mockRemoveDropUserError(database *fnv1alpha1.Database, client client.Client) (*sql.DB, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, err
	}

	mock.ExpectExec("DROP DATABASE IF EXISTS*").
		WillReturnResult(sqlmock.NewResult(1, 1))

	error := mysql.MySQLError{Number: 1, Message: "Something went wrong"}
	mock.ExpectExec("DROP USER*").
		WillReturnError(&error)

	mock.ExpectClose()
	return db, err
}

func mockRemoveGetAdminDatabaseConnection(database *fnv1alpha1.Database, client client.Client) (*sql.DB, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, err
	}

	mock.ExpectExec("DROP DATABASE IF EXISTS*").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mysql error code 1396 if user existence
	error := mysql.MySQLError{Number: 1396, Message: "User doesn't exists"}
	mock.ExpectExec("DROP USER*").
		WillReturnError(&error)

	mock.ExpectClose()
	return db, err
}
