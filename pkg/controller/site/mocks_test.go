package site

import (
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/testhelpers"
)

const (
	testNamespace       = "wlgore-prod"
	testApplicationName = "wlgore"
	testEnvironmentName = "wlgore-prod"
	testSiteName        = "default"
	testSecondSiteName  = "second"

	testDomain1           = "wlgore-prod-site1.com"
	testDomain2           = "wlgore-prod-site2.com"
	testDomain3           = "wlgore-prod-site3.com"
	testSecondSiteDomain1 = "second-site1.com"
	testSecondSiteDomain2 = "second-site2.com"

	testDatabaseName           = "wlgore-database"
	testDatabaseResourceName   = "wlgore-database-resource"
	testDatabaseUser           = "wlgore-database-user"
	testDatabaseHost           = "mysql"
	testDatabasePort           = 3306
	testDatabaseAdminSecret    = "wlgore-admin-secret"
	testDatabaseUserSecretName = "wlgore-user-secret"

	test2ndDatabaseName           = "second-database"
	test2ndDatabaseResourceName   = "second-database-resource"
	test2ndDatabaseUser           = "second-database-user"
	test2ndDatabaseHost           = "mysql-second"
	test2ndDatabasePort           = 33306
	test2ndDatabaseUserSecretName = "second-user-secret"
)

var (
	testAppID   = "0b48e3fe-09a1-4d96-8447-6198114e5d58"
	testEnvID   = "1c1f2619-4ec0-416f-bc32-09f57242082d"
	testSiteID  = "ae622f34-ac70-44d5-aec4-bb6d5dcd6d41"
	testSite2ID = "22222222-ac70-44d5-aec4-222222222222"

	testCreationTimestamp = metav1.Time{
		Time: time.Date(2019, time.November, 11, 0, 0, 0, 0, time.UTC),
	}

	testSiteLabels = map[string]string{
		fnv1alpha1.ApplicationIdLabel: testAppID,
		fnv1alpha1.EnvironmentIdLabel: testEnvID,
		fnv1alpha1.SiteIdLabel:        testSiteID,
	}

	testSite = &fnv1alpha1.Site{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testSiteName,
			Namespace:         testNamespace,
			CreationTimestamp: testCreationTimestamp,
			Labels:            testSiteLabels,
		},
		Spec: fnv1alpha1.SiteSpec{
			Environment: testEnvironmentName,
			Domains: []string{
				testDomain1,
			},
		},
	}

	// A DrupalApplication resource.
	drupalApplication = &fnv1alpha1.DrupalApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testApplicationName,
			Namespace: testNamespace,
		},
	}

	testDatabase = &fnv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDatabaseResourceName,
			Namespace: testNamespace,
		},
		Spec: fnv1alpha1.DatabaseSpec{
			Host:        testDatabaseHost,
			Port:        testDatabasePort,
			SchemaName:  testDatabaseName,
			User:        testDatabaseUser,
			UserSecret:  testDatabaseUserSecretName,
			AdminSecret: testDatabaseAdminSecret,
		},
	}

	test2ndDatabase = &fnv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      test2ndDatabaseResourceName,
			Namespace: testNamespace,
		},
		Spec: fnv1alpha1.DatabaseSpec{
			Host:        test2ndDatabaseHost,
			Port:        test2ndDatabasePort,
			SchemaName:  test2ndDatabaseName,
			User:        test2ndDatabaseUser,
			UserSecret:  test2ndDatabaseUserSecretName,
			AdminSecret: testDatabaseAdminSecret,
		},
	}

	testDBUserSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDatabaseUserSecretName,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"password": []byte("testpassword"),
		},
	}

	test2ndDBUserSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      test2ndDatabaseUserSecretName,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"password": []byte("2ndpassword"),
		},
	}

	dbAdminSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDatabaseAdminSecret,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"password": []byte("testadminpassword"),
			"username": []byte("testadminuser"),
		},
	}

	drupalEnvironment = &fnv1alpha1.DrupalEnvironment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testEnvironmentName,
			Namespace: testNamespace,
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
				fnv1alpha1.EnvironmentIdLabel: testEnvID,
			},
		},
		Spec: fnv1alpha1.DrupalEnvironmentSpec{
			Application: testApplicationName,
		},
	}

	siteWithoutID = &fnv1alpha1.Site{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSiteName,
			Namespace: testNamespace,
		},
	}

	// A Site resource with metadata and spec.
	siteWithID = &fnv1alpha1.Site{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSiteName,
			Namespace: testNamespace,
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
				fnv1alpha1.EnvironmentIdLabel: testEnvID,
				fnv1alpha1.SiteIdLabel:        testSiteID,
			},
		},
		Spec: fnv1alpha1.SiteSpec{
			Environment: testEnvironmentName,
			Domains: []string{
				testDomain1,
				testDomain2,
			},
			Database: testDatabaseResourceName,
		},
	}

	// A Second Site resource with metadata and spec.
	testSecondSite = &fnv1alpha1.Site{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSecondSiteName,
			Namespace: testNamespace,
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
				fnv1alpha1.EnvironmentIdLabel: testEnvID,
				fnv1alpha1.SiteIdLabel:        testSite2ID,
			},
		},
		Spec: fnv1alpha1.SiteSpec{
			Environment: testEnvironmentName,
			Domains: []string{
				testSecondSiteDomain1,
				testSecondSiteDomain2,
			},
			Database: test2ndDatabaseResourceName,
		},
	}
)

// buildFakeReconcile return reconcile with fake client, schemes and runtime objects
func buildFakeReconcile(objects []runtime.Object) *ReconcileSite {
	c := testhelpers.NewFakeClient(objects)

	// create a ReconcileDrupalApplication object with the scheme and fake client
	return &ReconcileSite{
		client: c,
		scheme: scheme.Scheme,
	}
}
