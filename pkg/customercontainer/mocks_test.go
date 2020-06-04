package customercontainer

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
)

const (
	testEnvironmentName = "wlgore-app-prod"
	testNamespace       = "wlgore-app-prod"
	testApplicationName = "wlgore-app"
	testGitRepo         = "gitlab.fn.acquia.io/wlgore/poc-gore.git"
	testImageRepo       = "881217801864.dkr.ecr.us-east-1.amazonaws.com/customer/gitlab.fn.acquia.io/wlgore-app/poc-gore"
)

var (
	testAppID          = "c7b96d1a-e50a-47f2-a94b-f1f6aada4704"
	testEnvID          = "d6a1c503-c2b0-48d7-8d64-450cdfcb07ee"
	testCPUUtilization = int32(50)

	drupalEnvironmentWithID = &fnv1alpha1.DrupalEnvironment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testEnvironmentName,
			Namespace: testNamespace,
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
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
)
