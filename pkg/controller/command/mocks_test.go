package command

import (
	"time"

	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/testhelpers"
)

const (
	namespace           = "wlgore-prod"
	testCommandName     = "command-wlgore"
	testApplicationName = "wlgore"
	testEnvironmentName = "wlgore-prod"
	testSiteName        = "wlgore-prod-site"
	testDomain          = "wlgore-prod-site1.com"

	testImage = "test-image:latest"
	schedule  = "* * * * *"

	customerCodeImage = "customer/code:latest"
	phpFpmImage       = "php-fpm:latest"
)

var (
	testAppID  = "0b48e3fe-09a1-4d96-8447-6198114e5d58"
	testEnvID  = "1c1f2619-4ec0-416f-bc32-09f57242082d"
	testSiteID = "ae622f34-ac70-44d5-aec4-bb6d5dcd6d41"

	testCreationTimestamp = metav1.Time{
		Time: time.Date(2019, time.November, 11, 0, 0, 0, 0, time.UTC),
	}
	testTimeNow = metav1.Time{
		Time: time.Date(2019, time.November, 11, 12, 0, 0, 0, time.UTC),
	}
	testTimeThen = metav1.Time{
		Time: time.Date(2019, time.November, 11, 11, 0, 0, 0, time.UTC),
	}

	siteLabels = map[string]string{
		fnv1alpha1.ApplicationIdLabel: testAppID,
		fnv1alpha1.EnvironmentIdLabel: testEnvID,
		fnv1alpha1.SiteIdLabel:        testSiteID,
	}

	activeDeadline         = int64(123)
	terminationGracePeriod = int64(456)

	additionalLabels = map[string]string{
		"test-1": "foo",
		"test-2": "bar",
	}

	additionalEnvVars = []corev1.EnvVar{
		{Name: "foo", Value: "bar"},
		{Name: "bim", Value: "bap"},
	}

	additionalVolumes = []corev1.Volume{
		{
			Name: "test-vol",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	additionalVolumeMounts = []corev1.VolumeMount{
		{
			Name:      "test-vol",
			MountPath: "/test",
		},
	}

	overrideResouces = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("333Mi"),
		},
	}

	defaultCommandOnSite = &fnv1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCommandName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":     "drupal",
				"command": "drush",
			},
		},
		Spec: fnv1alpha1.CommandSpec{
			TargetRef: fnv1alpha1.TargetRef{
				APIVersion: "fnresources.acquia.io/v1alpha1",
				Kind:       "Site",
				Name:       testSiteName,
			},
			Command: []string{"drush", "st"},
		},
	}

	rootCommandOnEnv = &fnv1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCommandName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":     "drupal",
				"command": "drush",
			},
		},
		Spec: fnv1alpha1.CommandSpec{
			TargetRef: fnv1alpha1.TargetRef{
				APIVersion: "fnresources.acquia.io/v1alpha1",
				Kind:       "DrupalEnvironment",
				Name:       testEnvironmentName,
			},
			Command:                       []string{"mkdir", "/foo"},
			Retries:                       1,
			RunAsRoot:                     true,
			Resources:                     &overrideResouces,
			ActiveDeadlineSeconds:         &activeDeadline,
			TerminationGracePeriodSeconds: &terminationGracePeriod,
			Image:                         testImage,
			AdditionalLabels:              additionalLabels,
			AdditionalEnvVars:             additionalEnvVars,
			AdditionalVolumes:             additionalVolumes,
			AdditionalVolumeMounts:        additionalVolumeMounts,
		},
	}

	defaultCronCommand = &fnv1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCommandName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":     "drupal",
				"command": "drush",
			},
		},
		Spec: fnv1alpha1.CommandSpec{
			TargetRef: fnv1alpha1.TargetRef{
				APIVersion: "fnresources.acquia.io/v1alpha1",
				Kind:       "Site",
				Name:       testSiteName,
			},
			Command:  []string{"drush", "cron"},
			Schedule: schedule,
		},
	}

	rootCronCommand = &fnv1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCommandName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":     "drupal",
				"command": "drush",
			},
		},
		Spec: fnv1alpha1.CommandSpec{
			TargetRef: fnv1alpha1.TargetRef{
				APIVersion: "fnresources.acquia.io/v1alpha1",
				Kind:       "Site",
				Name:       testSiteName,
			},
			Command:                       []string{"drush", "cron"},
			Retries:                       1,
			RunAsRoot:                     true,
			Schedule:                      schedule,
			Suspend:                       true,
			ConcurrencyPolicy:             batchv1beta1.AllowConcurrent,
			Resources:                     &overrideResouces,
			ActiveDeadlineSeconds:         &activeDeadline,
			TerminationGracePeriodSeconds: &terminationGracePeriod,
			Image:                         testImage,
			AdditionalLabels:              additionalLabels,
			AdditionalEnvVars:             additionalEnvVars,
			AdditionalVolumes:             additionalVolumes,
			AdditionalVolumeMounts:        additionalVolumeMounts,
		},
	}

	drupalApplication = &fnv1alpha1.DrupalApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testApplicationName,
			Namespace:         namespace,
			CreationTimestamp: testCreationTimestamp,
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
			},
		},
	}

	drupalEnvironment = &fnv1alpha1.DrupalEnvironment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testEnvironmentName,
			Namespace:         namespace,
			CreationTimestamp: testCreationTimestamp,
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
				fnv1alpha1.EnvironmentIdLabel: testEnvID,
			},
		},
		Spec: fnv1alpha1.DrupalEnvironmentSpec{
			Application: testApplicationName,
		},
	}

	site = &fnv1alpha1.Site{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testSiteName,
			Namespace:         namespace,
			CreationTimestamp: testCreationTimestamp,
			Labels:            siteLabels,
		},
		Spec: fnv1alpha1.SiteSpec{
			Environment: testEnvironmentName,
			Domains: []string{
				testDomain,
			},
		},
	}

	drupalPod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "drupal-foo",
			Namespace: namespace,
			Labels: map[string]string{
				fnv1alpha1.ApplicationIdLabel: testAppID,
				fnv1alpha1.EnvironmentIdLabel: testEnvID,
				"app":                         "drupal",
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:  "code-copy",
					Image: customerCodeImage,
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "php-fpm",
					Image: phpFpmImage,
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "shared-files",
							MountPath: "/var/www/html/docroot/sites/default/files",
							SubPath:   "31805192-9bce-433b-8c5b-05c34f76e3b6-drupal-files",
						},
						{
							Name:      "drupal-code",
							MountPath: "/var/www/html/docroot/sites/default/files",
							SubPath:   "31805192-9bce-433b-8c5b-05c34f76e3b6-drupal-files",
						},
						{
							Name:      "default-token-9cszq",
							MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
							ReadOnly:  true,
						},
					},
				},
			},
		},
	}
)

// BuildFakeReconcile return reconcile with fake client, schemes and runtime objects
func BuildFakeReconcile(objects []runtime.Object) *ReconcileCommand {
	c := testhelpers.NewFakeClient(objects)

	// create a ReconcileDrupalApplication object with the scheme and fake client
	return &ReconcileCommand{
		client: c,
		scheme: scheme.Scheme,
	}
}
