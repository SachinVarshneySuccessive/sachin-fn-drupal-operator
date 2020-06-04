package command

import (
	"context"
	"fmt"
	"github.com/acquia/fn-drupal-operator/pkg/common"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fnresourcesv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
)

func (rh *requestHandler) generateJobParamsFromFnResources(apiVersion string) (jobParams jobParams, target metav1.Object, err error) {
	targetRef := rh.cmd.Spec.TargetRef
	// Only accept v1alpha1 API Version for now
	if apiVersion != "v1alpha1" {
		err = fmt.Errorf("unsupported targetRef API version '%v'", targetRef.APIVersion)
		return
	}

	// Call appropriate handler to populate jobParams
	switch targetRef.Kind {
	case "Site":
		return rh.handleSite(targetRef.Name)

	case "DrupalEnvironment":
		return rh.handleDrupalEnvironment(targetRef.Name)

	default:
		err = fmt.Errorf("unsupported targetRef Kind '%v'", targetRef.Kind)
		return
	}
}

func (rh *requestHandler) handleSite(name string) (jobParams jobParams, site *fnresourcesv1alpha1.Site, err error) {
	site = &fnresourcesv1alpha1.Site{}
	nn := types.NamespacedName{Namespace: rh.cmd.Namespace, Name: rh.cmd.Spec.TargetRef.Name}
	err = rh.r.client.Get(context.TODO(), nn, site)
	if err != nil {
		return
	}

	if jobParams, _, err = rh.handleDrupalEnvironment(site.Spec.Environment); err != nil {
		return
	}

	jobParams.labels = common.MergeLabels(rh.cmd.Labels, site.ChildLabels())

	// Add the site name to the jobParams env vars.
	jobParams.container.Env = append(jobParams.container.Env,
		corev1.EnvVar{Name: "AH_SITE_NAME", Value: site.Name})
	return
}

func (rh *requestHandler) handleDrupalEnvironment(name string) (jobParams jobParams, env *fnresourcesv1alpha1.DrupalEnvironment, err error) {
	env = &fnresourcesv1alpha1.DrupalEnvironment{}
	nn := types.NamespacedName{Namespace: rh.cmd.Namespace, Name: name}
	err = rh.r.client.Get(context.TODO(), nn, env)
	if err != nil {
		return
	}

	// Find a Pod to use for jobParams
	podList := &corev1.PodList{}
	labels := map[string]string{
		fnresourcesv1alpha1.EnvironmentIdLabel: string(env.Id()),
		"app":                                  "drupal",
	}

	err = rh.r.client.List(context.TODO(), podList, client.MatchingLabels(labels))
	if err != nil {
		rh.logger.Error(err, "Failed to List Pods by Environment ID", "Environment ID", env.Id())
		return
	}

	if len(podList.Items) == 0 {
		err = fmt.Errorf("couldn't find any 'drupal' Pods")
		return
	}
	pod := podList.Items[0]

	// Base the jobParams on the "php-fpm" container, but with Image from "code-copy" init container
	c := findContainerByName(pod.Spec.Containers, "php-fpm")
	if c == nil {
		err = fmt.Errorf("failed to find 'php-fpm' container in 'drupal' Pod %v", pod.UID)
		return
	}

	init := findContainerByName(pod.Spec.InitContainers, "code-copy")
	if init == nil {
		err = fmt.Errorf("failed to find 'code-copy' container in 'drupal' Pod %v", pod.UID)
		return
	}
	c.Image = init.Image

	// Remove "drupal-code" and default token volume mounts, since they might cause conflicts
	var volumeMounts []corev1.VolumeMount
	for _, vm := range c.VolumeMounts {
		if vm.Name != "drupal-code" && !strings.Contains(vm.Name, "default-token-") {
			volumeMounts = append(volumeMounts, vm)
		}
	}
	c.VolumeMounts = volumeMounts

	jobParams.container = *c
	jobParams.volumes = pod.Spec.Volumes
	jobParams.labels = common.MergeLabels(rh.cmd.Labels, env.ChildLabels())

	return
}

func findContainerByName(cs []corev1.Container, name string) *corev1.Container {
	for _, c := range cs {
		if c.Name == name {
			return &c
		}
	}
	return nil
}
