package customercontainer

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"
)

const (
	sharedFilesName = "shared-files"
)

var customerECR = "881217801864.dkr.ecr.us-east-1.amazonaws.com"
var customerECRRepoNamePrefix = "customer"
var gitRepoRegexp = regexp.MustCompile(`^(.*@|)([^/:]+)[:/]([^.]+)(.git|)$`)

func init() {
	envCustomerECR := os.Getenv("CUSTOMER_ECR")
	if envCustomerECR != "" {
		customerECR = envCustomerECR
	}

	envCustomerECRRepoNamePrefix := os.Getenv("CUSTOMER_ECR_REPO_NAME_PREFIX")
	if envCustomerECRRepoNamePrefix != "" {
		customerECRRepoNamePrefix = envCustomerECRRepoNamePrefix
	}
}

func prefix(e *fnv1alpha1.DrupalEnvironment) string {
	return string(e.Id())
}

func ImageName(a *fnv1alpha1.DrupalApplication, e *fnv1alpha1.DrupalEnvironment) (imageName string) {
	if a.Spec.ImageRepo == "" {
		imageName = fmt.Sprintf("%s:%s", ECRRepoURIFromGitRepoURL(a.Spec.GitRepo), e.Spec.Drupal.Tag)
	} else {
		imageName = fmt.Sprintf("%s:%s", a.Spec.ImageRepo, e.Spec.Drupal.Tag)
	}
	return
}

func EnvironmentVariables(e *fnv1alpha1.DrupalEnvironment) []v1.EnvVar {
	envType := "AH_NON_PRODUCTION"
	if e.Spec.Production == true {
		envType = "AH_PRODUCTION"
	}

	stdEnvVars := []v1.EnvVar{
		v1.EnvVar{Name: envType, Value: "1"},
		v1.EnvVar{Name: "AH_APPLICATION_UUID", Value: e.Labels[fnv1alpha1.ApplicationIdLabel]},
		v1.EnvVar{Name: "AH_CURRENT_REGION", Value: common.AwsRegion()},
		v1.EnvVar{Name: "AH_REALM", Value: common.Realm()},
		v1.EnvVar{Name: "AH_SITE_ENVIRONMENT", Value: e.Spec.Stage},
		v1.EnvVar{Name: "AH_SITE_GROUP", Value: e.Spec.Application},
		v1.EnvVar{Name: "TEMP", Value: "/shared/tmp"},
		v1.EnvVar{Name: "TMPDIR", Value: "/shared/tmp"},
	}
	return append(
		e.Spec.CustomEnvironmentVariables,
		stdEnvVars...,
	)
}

func ApacheEnvironmentVariables(e *fnv1alpha1.DrupalEnvironment) []v1.EnvVar {
	return append(
		EnvironmentVariables(e),
		v1.EnvVar{
			Name:  "DOCROOT",
			Value: "/var/www/html/" + e.Spec.Apache.WebRoot,
		},
	)
}

func FilesVolume(e *fnv1alpha1.DrupalEnvironment) v1.Volume {
	return v1.Volume{
		Name: sharedFilesName,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: prefix(e) + "-files",
			},
		},
	}
}

func FilesVolumeMount(e *fnv1alpha1.DrupalEnvironment) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      sharedFilesName,
		MountPath: "/var/www/html/docroot/sites/default/files", // FIXME: https://backlog.acquia.com/browse/NW-98 - add support for multisite
		SubPath:   prefix(e) + "-drupal-files",
	}
}

func SharedVolumeMount(e *fnv1alpha1.DrupalEnvironment) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      sharedFilesName,
		MountPath: "/shared",
		SubPath:   prefix(e) + "-shared",
	}
}

func Template(a *fnv1alpha1.DrupalApplication, e *fnv1alpha1.DrupalEnvironment) v1.Container {
	return v1.Container{
		Image:           ImageName(a, e),
		ImagePullPolicy: e.Spec.Drupal.PullPolicy,
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("200m"),
				v1.ResourceMemory: resource.MustParse("375Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("500m"),
				v1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
		Env: EnvironmentVariables(e),
		VolumeMounts: []v1.VolumeMount{
			FilesVolumeMount(e),
			SharedVolumeMount(e),
			{
				Name:      "php-config",
				MountPath: "/usr/local/php/etc/conf.d/zzz_drupalenvironment.ini",
				SubPath:   "zzz_drupalenvironment.ini",
				ReadOnly:  true,
			},
			{
				Name:      "php-config",
				MountPath: "/usr/local/php/etc/cli/conf.d/zzz_drupalenvironment_cli.ini",
				SubPath:   "zzz_drupalenvironment_cli.ini",
				ReadOnly:  true,
			},
			{
				Name:      "php-config",
				MountPath: "/usr/local/php/etc/conf.d/newrelic.ini",
				SubPath:   "newrelic.ini",
				ReadOnly:  true,
			},
			{
				Name:      "env-config",
				MountPath: "/mnt/env-config/",
				ReadOnly:  true,
			},
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
	}
}

// Normalize a Git Repo URL to a unique portion that can be used as part of an ECR repo name.
// https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_Repository.html#ECR-Type-Repository-repositoryName
func NormalizeGitPartialURL(repoURL string) string {
	return gitRepoRegexp.ReplaceAllString(repoURL, "$2/$3")
}

func ECRRepoNameFromGitRepoURL(gitURL string) string {
	return fmt.Sprintf("%v/%v", customerECRRepoNamePrefix, NormalizeGitPartialURL(gitURL))
}

func ECRRepoURIFromGitRepoURL(gitURL string) string {
	return fmt.Sprintf("%v/%v", customerECR, ECRRepoNameFromGitRepoURL(gitURL))
}

func CustomerECR() string {
	return customerECR
}

// true if the given ECR repo is in the the customer repo portion of the ECR registry.
func IsCustomerECRRepo(ecrRepoName string) bool {
	return strings.HasPrefix(ecrRepoName, customerECRRepoNamePrefix+"/")
}

// returns the git repo URL corresponding to the given ECR repo name.
func GitPartialURLFromECRRepoName(ecrRepoName string) string {
	repoURL := ecrRepoName[len(customerECRRepoNamePrefix)+1:]
	return repoURL
}
