package drupalenvironment

import (
	"time"

	"github.com/acquia/fn-ssh-proxy/pkg/sshtunnel"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/testhelpers"
)

func getDbSecret() map[string]string {
	return map[string]string{
		"host":     "mysql-server.default.svc.cluster.local",
		"password": "admin",
		"port":     "3306",
		"username": "root",
	}
}

const (
	testEnvironmentName = "wlgore-app-prod"
	testNamespace       = "wlgore-app-prod"
	testApplicationName = "wlgore-app"
	testSiteName        = "default"
	testSecondSiteName  = "site2"

	testDomain1 = "default1.wlgore-prod.com"
	testDomain2 = "default2.wlgore-prod.com"
	testDomain3 = "second1.wlgore-prod.com"
	testDomain4 = "second2.wlgore-prod.com"

	testDatabaseResourceName       = "wlgore-default"
	testSecondDatabaseResourceName = "wlgore-second"

	testSshUsername                 = "test"
	testAuthorizedKeysConfigMapName = "ssh-authorized-keys"

	testGitRepo   = "gitlab.fn.acquia.io/wlgore/poc-gore.git"
	testImageRepo = "881217801864.dkr.ecr.us-east-1.amazonaws.com/customer/gitlab.fn.acquia.io/wlgore-app/poc-gore"

	testNonProdEnvironmentName = "wlgore-app-non-prod"
	testNonProdNamespace       = "wlgore-app-non-prod"

	testNewRelicSecretName = "newrelic"
)

var (
	testAppID       = "c7b96d1a-e50a-47f2-a94b-f1f6aada4704"
	testEnvID       = "d6a1c503-c2b0-48d7-8d64-450cdfcb07ee"
	testSiteID      = "ae622f34-ac70-44d5-aec4-bb6d5dcd6d41"
	testSecondEnvID = "560fc690-4e5c-41d2-8fee-bef00c5c9693"

	testCPUUtilization = int32(50)

	testCreationTimestamp = metav1.Time{
		Time: time.Date(2019, time.November, 11, 0, 0, 0, 0, time.UTC),
	}

	testNamespaceResource = &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}

	testNonProdNamespaceResource = &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNonProdNamespace,
		},
	}

	// A DrupalEnvironment resource with metadata and spec.
	drupalEnvironmentWithoutID = &fnv1alpha1.DrupalEnvironment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testEnvironmentName,
			Namespace: testNamespace,
		},
	}

	drupalEnvironmentWithID = &fnv1alpha1.DrupalEnvironment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testEnvironmentName,
			Namespace: testNamespace,
			Labels: map[string]string{
				fnv1alpha1.EnvironmentIdLabel: testEnvID,
				fnv1alpha1.VersionLabel:       "2",
			},
		},
		Spec: fnv1alpha1.DrupalEnvironmentSpec{
			Application: testApplicationName,
			GitRef:      "refs/heads/master",
			CustomEnvironmentVariables: []v1.EnvVar{
				{
					Name:  "CUSTOM_ENV1",
					Value: "testValue1",
				},
				{
					Name:  "CUSTOM_ENV2",
					Value: "testValue2",
				}},
			Stage:      "prod",
			Production: true,
			Phpfpm: fnv1alpha1.SpecPhpFpm{
				Tag:                             "7.3",
				Procs:                           4,
				ProcMemoryLimitMiB:              128,
				PostMaxSizeMiB:                  8,
				OpcacheMemoryLimitMiB:           96,
				OpcacheInternedStringsBufferMiB: 8,
				ApcMemoryLimitMiB:               32,
				MaxInputVars:                    1000,
				MaxExecutionTime:                30,
				NewRelicAppName:                 "Test App",
				NewRelicSecret:                  testNewRelicSecretName,
			},
			Drupal: fnv1alpha1.SpecDrupal{
				MinReplicas:                    2,
				MaxReplicas:                    2,
				Tag:                            "1.0.0",
				PullPolicy:                     "Always",
				TargetCPUUtilizationPercentage: &testCPUUtilization,

				Liveness: fnv1alpha1.HTTPProbe{
					Enabled:          true,
					HTTPPath:         "/user/login",
					TimeoutSeconds:   5,
					FailureThreshold: 5,
					SuccessThreshold: 1,
					PeriodSeconds:    10,
				},
				Readiness: fnv1alpha1.HTTPProbe{
					Enabled:          true,
					HTTPPath:         "/user/login",
					TimeoutSeconds:   5,
					FailureThreshold: 5,
					SuccessThreshold: 1,
					PeriodSeconds:    10,
				},
			},
			Apache: fnv1alpha1.SpecApache{
				Tag:     "latest",
				WebRoot: "docroot",
			},
		},
	}

	drupalEnvironmentWithNonProdValues = &fnv1alpha1.DrupalEnvironment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNonProdEnvironmentName,
			Namespace: testNonProdNamespace,
			Labels: map[string]string{
				fnv1alpha1.EnvironmentIdLabel: testSecondEnvID,
				fnv1alpha1.VersionLabel:       "2",
			},
		},
		Spec: fnv1alpha1.DrupalEnvironmentSpec{
			Application: testApplicationName,
			GitRef:      "refs/heads/master",
			CustomEnvironmentVariables: []v1.EnvVar{
				{
					Name:  "CUSTOM_ENV1",
					Value: "testValue1",
				},
				{
					Name:  "CUSTOM_ENV2",
					Value: "testValue2",
				}},
			Stage:      "dev",
			Production: false,
			Phpfpm: fnv1alpha1.SpecPhpFpm{
				Tag:                             "7.3",
				Procs:                           4,
				ProcMemoryLimitMiB:              128,
				PostMaxSizeMiB:                  8,
				OpcacheMemoryLimitMiB:           96,
				OpcacheInternedStringsBufferMiB: 8,
				ApcMemoryLimitMiB:               32,
				MaxInputVars:                    1000,
				MaxExecutionTime:                30,
				NewRelicSecret:                  testNewRelicSecretName,
			},
			Drupal: fnv1alpha1.SpecDrupal{
				MinReplicas:                    2,
				MaxReplicas:                    2,
				Tag:                            "1.0.0",
				PullPolicy:                     "Always",
				TargetCPUUtilizationPercentage: &testCPUUtilization,

				Liveness: fnv1alpha1.HTTPProbe{
					Enabled:          true,
					HTTPPath:         "/user/login",
					TimeoutSeconds:   5,
					FailureThreshold: 5,
					SuccessThreshold: 1,
					PeriodSeconds:    10,
				},
				Readiness: fnv1alpha1.HTTPProbe{
					Enabled:          true,
					HTTPPath:         "/user/login",
					TimeoutSeconds:   5,
					FailureThreshold: 5,
					SuccessThreshold: 1,
					PeriodSeconds:    10,
				},
			},
			Apache: fnv1alpha1.SpecApache{
				Tag:     "latest",
				WebRoot: "docroot",
			},
		},
	}

	drupalApplicationWithID = &fnv1alpha1.DrupalApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: testApplicationName,
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
			},
		},
		Spec: fnv1alpha1.DrupalApplicationSpec{
			ImageRepo: testImageRepo,
			GitRepo:   testGitRepo,
		},
	}

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

	testSecondSite = &fnv1alpha1.Site{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSecondSiteName,
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
				testDomain3,
				testDomain4,
			},
			Database: testSecondDatabaseResourceName,
		},
	}

	testDefaultClusterCredsSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-cluster-creds",
			Namespace: "default",
		},
		StringData: getDbSecret(),
	}

	testProdNewRelicSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNewRelicSecretName,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			newRelicLicenseSecretKey: []byte("1234567890abcdef"),
		},
	}

	testNonProdNewRelicSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testNewRelicSecretName,
			Namespace: testNonProdNamespace,
		},
		Data: map[string][]byte{
			newRelicLicenseSecretKey: []byte("3333333333333333"),
		},
	}

	testSshAuthorizedKeysConfigMap = &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAuthorizedKeysConfigMapName,
			Namespace: testNamespace,
			Labels: map[string]string{
				sshtunnel.LabelSshUser: testSshUsername,
			},
		},
		Data: map[string]string{
			"keys": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDE6Wi8op1I6vcIpjcivdvCDiH40MbfQvuMkU+E1WI/th+fZIIM0ThgdQ+UsnAGTcPeFJPG/fHmDc5SB63spYzFNM/4Upadpxz5aG3VuIFvPErLvpWexpQKv74X60NeaaqoQLiK4+ibho0mH58Zn1a3nzdxabF3vUzcvFs94nHC2V8xXMeO6Xa2JcKOXCj8tXHNePAFHE1DLs1aMXpCuQNz++YVohGHsq8WW+6w3DoJ2SF0eiRVjwB6hRo7R0yssmWKe+aIHyybocEVWQarnDAlpsqXXl7CWkh9hK2IJUVc+bf0STdjX4qecqQIushW8V32l2GrwFQsS+2xjQsXeWEvyG0DEtsWtaju/ZaBe8/E+cZRMicRUE5sccNbe9MRR6IUTM5joU95EkRkQAfEzToEj3XRjMoTLqyi3F9qAg/ILXDu2TurUDuucK3t69oidMeNJ8GPu24zS0AXkNuqKKSqmblf/OwZsiYF7LW2dltuETdnb9CXUM43WmWhSWWrd30sJC8mLDROoCmhKXOqOBFbhJpgpncNhiJMrPz5hIB5iJreQKIpP/v5uEqKY0tbFn8euKBWaZrDmfNQV5BdSE2PbvsxtgHxvv47gCvK3re/kX4C7rtP+2lI+8QuyOawka4vyhvA070fkDOzrSUjV23geTRGiBB27dQjpU1a7rX7Pw== test",
		},
	}
)

// buildFakeReconcile return reconcile with fake client, schemes and runtime objects
func buildFakeReconcile(objects []runtime.Object) *ReconcileDrupalEnvironment {
	client := testhelpers.NewFakeClient(objects)

	// create a ReconcileDrupalEnvironment object with the scheme and fake client
	return &ReconcileDrupalEnvironment{
		client: client,
		scheme: scheme.Scheme,
	}
}
