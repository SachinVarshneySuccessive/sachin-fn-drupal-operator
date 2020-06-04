package drupalenvironment

import (
	"context"
	"fmt"
	"strings"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/go-test/deep"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"
	"github.com/acquia/fn-drupal-operator/pkg/customercontainer"
)

const (
	// phpMemoryOverprovisionFactor is the ratio of "memory requested" : "memory limit" for PHP-FPM containers
	phpMemoryOverprovisionFactor = 1.0 / 3.0

	drupalRolloutName   = "drupal"
	DrupalServiceName   = "drupal"
	phpFpmContainerName = "php-fpm"
	defaultCustomImage  = "default"

	ecrRoot = "881217801864.dkr.ecr.us-east-1.amazonaws.com" // TODO: env var
)

func drupalCodeMount(path string) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      "drupal-code",
		MountPath: path,
	}
}

func EnvConfigSecretVolume() v1.Volume {
	defaultMode := int32(0644)
	return v1.Volume{
		Name: "env-config",
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName:  "env-config",
				DefaultMode: &defaultMode, // to prevent recurring Update()s
			},
		},
	}
}

func PhpConfigVolume() v1.Volume {
	defaultMode := int32(0644)
	return v1.Volume{
		Name: "php-config",
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{Name: "php-config"},
				DefaultMode:          &defaultMode, // to prevent recurring Update()s
			},
		},
	}
}

func ApacheConfEnabled() v1.Volume {
	defaultMode := int32(0644)
	return v1.Volume{
		Name: "apache-conf-enabled",
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{Name: "apache-conf-enabled"},
				DefaultMode:          &defaultMode, // to prevent recurring Update()s
			},
		},
	}
}

func apacheContainer(env *fnv1alpha1.DrupalEnvironment) v1.Container {
	drupal := env.Spec.Drupal

	customImage := defaultCustomImage
	if env.Spec.Apache.CustomImage != "" {
		customImage = env.Spec.Apache.CustomImage
	}

	apacheContainer := v1.Container{
		Name:            "apache",
		Image:           fmt.Sprintf("%v/apache/%v:%v", ecrRoot, customImage, env.Spec.Apache.Tag),
		ImagePullPolicy: drupal.PullPolicy,
		Ports: []v1.ContainerPort{{
			ContainerPort: 8080,
			Name:          "http",
		}},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    env.Spec.Apache.Cpu.Request,
				v1.ResourceMemory: env.Spec.Apache.Memory.Request,
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    env.Spec.Apache.Cpu.Limit,
				v1.ResourceMemory: env.Spec.Apache.Memory.Limit,
			},
		},
		Env: customercontainer.ApacheEnvironmentVariables(env),
		VolumeMounts: []v1.VolumeMount{
			drupalCodeMount("/var/www"),
			customercontainer.FilesVolumeMount(env),
			{
				Name:      "apache-conf-enabled",
				MountPath: "/etc/apache2/conf-enabled/passenv.conf",
				SubPath:   "passenv.conf",
				ReadOnly:  true,
			},
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
	}

	if drupal.Liveness.Enabled {
		apacheContainer.LivenessProbe = &v1.Probe{
			Handler: v1.Handler{
				HTTPGet: &v1.HTTPGetAction{
					Path:        drupal.Liveness.HTTPPath,
					Port:        intstr.FromString("http"),
					Scheme:      "HTTP",
					HTTPHeaders: []v1.HTTPHeader{{Name: "Host", Value: "localhost"}}, // "Spoof" host so Drupal trusted hosts settings don't reject
				},
			},
			SuccessThreshold:    drupal.Liveness.SuccessThreshold,
			FailureThreshold:    drupal.Liveness.FailureThreshold,
			TimeoutSeconds:      drupal.Liveness.TimeoutSeconds,
			PeriodSeconds:       drupal.Liveness.PeriodSeconds,
			InitialDelaySeconds: 1,
		}
	}

	if drupal.Readiness.Enabled {
		apacheContainer.ReadinessProbe = &v1.Probe{
			Handler: v1.Handler{
				HTTPGet: &v1.HTTPGetAction{
					Path:        drupal.Readiness.HTTPPath,
					Port:        intstr.FromString("http"),
					Scheme:      "HTTP",
					HTTPHeaders: []v1.HTTPHeader{{Name: "Host", Value: "localhost"}}, // "Spoof" host so Drupal trusted hosts settings don't reject
				},
			},
			SuccessThreshold:    drupal.Readiness.SuccessThreshold,
			FailureThreshold:    drupal.Readiness.FailureThreshold,
			TimeoutSeconds:      drupal.Readiness.TimeoutSeconds,
			PeriodSeconds:       drupal.Readiness.PeriodSeconds,
			InitialDelaySeconds: 1,
		}
	}

	return apacheContainer
}

func (rh *requestHandler) phpFpmContainer() v1.Container {
	phpfpm := rh.env.Spec.Phpfpm

	phpMemoryLimit := int64(
		phpfpm.Procs*phpfpm.ProcMemoryLimitMiB+
			phpfpm.OpcacheMemoryLimitMiB+
			phpfpm.OpcacheInternedStringsBufferMiB+
			phpfpm.ApcMemoryLimitMiB,
	) * 1024 * 1024

	customImage := defaultCustomImage
	if phpfpm.CustomImage != "" {
		customImage = phpfpm.CustomImage
	}

	phpFpmContainer := customercontainer.Template(rh.app, rh.env)

	phpFpmContainer.Name = phpFpmContainerName
	phpFpmContainer.Image = fmt.Sprintf("%v/php-fpm/%v:%v", ecrRoot, customImage, phpfpm.Tag)

	phpFpmContainer.Resources = v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    phpfpm.Cpu.Request,
			v1.ResourceMemory: *resource.NewQuantity(int64(float64(phpMemoryLimit)*phpMemoryOverprovisionFactor), resource.BinarySI),
		},
		Limits: v1.ResourceList{
			v1.ResourceCPU:    phpfpm.Cpu.Limit,
			v1.ResourceMemory: *resource.NewQuantity(phpMemoryLimit, resource.BinarySI),
		},
	}
	phpFpmContainer.VolumeMounts = append(phpFpmContainer.VolumeMounts,
		v1.VolumeMount{
			Name:      "php-fpm-config",
			MountPath: "/usr/local/php/etc/php-fpm.d/",
			ReadOnly:  true,
		},
		drupalCodeMount("/var/www"),
	)

	return phpFpmContainer
}

func (rh *requestHandler) drupalRolloutSpec() rolloutsv1alpha1.RolloutSpec {
	ls := labelsForRollout(rh.env)
	rolloutAutoPromote := true // TODO: may not want to auto-promote in a multisite configuration
	rolloutAutoPromoteDelay := int32(10)
	// userReadOnly := int32(0400)
	twoReplicas := int32(2)
	scaleDownDelay := int32(30) // see https://github.com/argoproj/argo-rollouts/issues/19#issuecomment-476329960

	return rolloutsv1alpha1.RolloutSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: ls,
		},
		Strategy: rolloutsv1alpha1.RolloutStrategy{
			BlueGreenStrategy: &rolloutsv1alpha1.BlueGreenStrategy{
				ActiveService:         DrupalServiceName,
				AutoPromotionEnabled:  &rolloutAutoPromote,
				AutoPromotionSeconds:  &rolloutAutoPromoteDelay,
				ScaleDownDelaySeconds: &scaleDownDelay,
			},
		},
		Replicas: &twoReplicas, // This field will actually be controlled by the HPA; this is just an initial value
		Template: rh.drupalPodTemplate(),
	}
}

func (rh *requestHandler) drupalPodTemplate() v1.PodTemplateSpec {
	podLabels := labelsForRollout(rh.env)
	annotations := drupalPodAnnotations(rh.env)
	rootUser := int64(0)
	defaultMode := int32(0644)
	filesVolumeMount := customercontainer.FilesVolumeMount(rh.env)

	// We set a label here in this case so that if istio becomes disabled later,
	// The pods will get cycled.  This is one necessary step in converting between
	// an itio-enabled system and a non-istio-enabled system.  THIS DOES NOT GUARANTEE
	// THAT THERE WILL BE NO PROBLEMS.  This simply takes care of one known challenge.
	if common.IsIstioEnabled() {
		podLabels["istio-enabled"] = "true"
	}

	drupal := rh.env.Spec.Drupal

	phpfpmConfigMap := v1.ConfigMapVolumeSource{
		LocalObjectReference: v1.LocalObjectReference{Name: "phpfpm-config"},
		DefaultMode:          &defaultMode, // to prevent recurring Update()s
	}

	codeCopyContainer := v1.Container{
		Name:            "code-copy",
		Image:           customercontainer.ImageName(rh.app, rh.env),
		ImagePullPolicy: drupal.PullPolicy,
		Command: []string{
			"rsync", "--stats", "--archive", "/var/www/html", "/drupal-code",
		},
		VolumeMounts: []v1.VolumeMount{
			drupalCodeMount("/drupal-code"),
		},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("100m"),
				v1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("500m"),
				v1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
	}

	sharedSetupContainer := v1.Container{
		Name:            "shared-setup",
		Image:           customercontainer.ImageName(rh.app, rh.env),
		ImagePullPolicy: drupal.PullPolicy,
		Command: []string{
			"/bin/sh", "-c",
			"mkdir -p /shared/php_sessions /shared/tmp /shared/config/sync /shared/private-files" +
				" && chown clouduser:clouduser /shared/* " + filesVolumeMount.MountPath,
		},
		SecurityContext: &v1.SecurityContext{
			RunAsUser: &rootUser,
		},
		VolumeMounts: []v1.VolumeMount{
			customercontainer.SharedVolumeMount(rh.env),
			filesVolumeMount,
		},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("100m"),
				v1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("500m"),
				v1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
	}

	// Apache
	apacheContainer := apacheContainer(rh.env)

	// PhpFpm
	phpFpmContainer := rh.phpFpmContainer()

	return v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      podLabels,
			Annotations: annotations,
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				codeCopyContainer,
				sharedSetupContainer,
			},
			Containers: []v1.Container{
				phpFpmContainer,
				apacheContainer,
			},
			NodeSelector: map[string]string{
				common.WorkerNodeLabel: "true",
			},
			Volumes: []v1.Volume{
				customercontainer.FilesVolume(rh.env),
				{
					Name:         "drupal-code",
					VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
				},
				{
					Name:         "php-fpm-config",
					VolumeSource: v1.VolumeSource{ConfigMap: &phpfpmConfigMap},
				},
				PhpConfigVolume(),
				EnvConfigSecretVolume(),
				ApacheConfEnabled(),
			},
		},
	}
}

// labelsForRollout returns the labels for selecting the resources
// belonging to the given DrupalEnvironment CR name.
func labelsForRollout(drupalEnv *fnv1alpha1.DrupalEnvironment) map[string]string {
	labels := drupalEnv.ChildLabels()
	labels["app"] = "drupal"
	return labels
}

// drupalPodAnnotations return the annotations for "drupal" Pods
func drupalPodAnnotations(env *fnv1alpha1.DrupalEnvironment) map[string]string {
	return map[string]string{
		// Hash all fields that affect configuration files, which would necessitate a forced Pod rotation to reload
		fnv1alpha1.ConfigHashAnnotation: common.HashValueOf(
			env.Spec.Apache.WebRoot,
			env.Spec.Phpfpm,
		),
	}
}

func (rh *requestHandler) drupalService(name string) *v1.Service {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromString("http"),
			}},
			Selector: labelsForRollout(rh.env),
		},
	}
	// Set DrupalEnvironment instance as the owner and controller
	rh.associateResourceWithController(svc)
	return svc
}

func (rh *requestHandler) pv(name string) (pv *v1.PersistentVolume, err error) {
	volumeMode := v1.PersistentVolumeFilesystem

	pv = &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		Spec: v1.PersistentVolumeSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			Capacity: v1.ResourceList{
				v1.ResourceStorage: resource.MustParse("128Mi"),
			},
			VolumeMode: &volumeMode,
		},
	}

	sc := common.DefaultStorageClass()
	switch sc {
	case "efs":
		pv.Spec.StorageClassName = "efs"
		pv.Spec.PersistentVolumeSource = v1.PersistentVolumeSource{
			CSI: &v1.CSIPersistentVolumeSource{
				Driver:       "efs.csi.aws.com",
				VolumeHandle: rh.env.Spec.EFSID,
			},
		}
	case "manual":
		directoryOrCreate := v1.HostPathDirectoryOrCreate
		pv.Spec.StorageClassName = "manual"
		pv.Spec.HostPath = &v1.HostPathVolumeSource{
			Path: "/var/local/microk8s-storage/" + rh.env.Spec.EFSID,
			Type: &directoryOrCreate,
		}
	default:
		err = fmt.Errorf("unsupported StorageClass")
		rh.logger.Error(err, "Unsupported StorageClass", "StorageClass", sc)
		return nil, err
	}

	return pv, nil
}

func (rh *requestHandler) pvc(name string) *v1.PersistentVolumeClaim {
	storageClass := common.DefaultStorageClass()

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("128Mi"),
				},
			},
		},
	}
	if !common.UseDynamicProvisioning() {
		pvc.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: rh.env.ChildLabels(),
		}
	}
	// Set DrupalEnvironment instance as the owner and controller
	rh.associateResourceWithController(pvc)
	return pvc
}

// reconcilePhpFpmConfigMap reconciles the "phpfpm-config" ConfigMap for the requested
// DrupalEnvironment. This ConfigMap contains ".conf" files that will be enabled on the php-fpm container.
func (rh *requestHandler) reconcilePhpFpmConfigMap() (result reconcile.Result, err error) {
	var conf strings.Builder
	fmt.Fprintln(&conf, "[www]")
	fmt.Fprintln(&conf, "pm.max_children =", rh.env.Spec.Phpfpm.Procs)

	if common.MeetsVersionConstraint(">= 7.3", rh.env.Spec.Phpfpm.Tag) {
		fmt.Fprintln(&conf, "[global]")
		fmt.Fprintln(&conf, "log_limit = 8192")
	}

	requeue, err := rh.reconcileConfigMap("phpfpm-config", map[string]string{
		"drupalenvironment.conf": conf.String(),
	})

	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	return
}

func (rh *requestHandler) reconcileDrupalRollout() (requeue bool, err error) {
	r := rh.reconciler

	rollout := &rolloutsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      drupalRolloutName,
			Namespace: rh.namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, rollout, func() error {
		desired := rh.drupalRolloutSpec()

		if rollout.ObjectMeta.CreationTimestamp.IsZero() {
			// Create
			rh.associateResourceWithController(rollout)
		}

		if diff := deep.Equal(rollout.Spec, desired); diff != nil {
			rh.logger.Info("Rollout Spec needs update", "current != desired", diff)
			rollout.Spec = desired
		}

		rollout.ObjectMeta.Labels = common.MergeLabels(rollout.ObjectMeta.Labels, labelsForRollout(rh.env))

		return nil
	})
	if err != nil {
		return false, err
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Reconciled Drupal Rollout", "operation", op)
		return true, nil
	}
	return false, nil
}

func (rh *requestHandler) reconcileDrupalService() (requeue bool, err error) {
	r := rh.reconciler
	name := "drupal"

	svc := rh.drupalService(name)

	found := &v1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: rh.namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		rh.logger.Info("Creating Service", "Namespace", svc.Namespace, "Name", svc.Name)
		err = r.client.Create(context.TODO(), svc)
		if err != nil {
			rh.logger.Error(err, "Failed to create Service", "Namespace", svc.Namespace, "Name", svc.Name)
			return false, err
		}
		return true, nil
	}
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile Service", "Namespace", rh.namespace, "Name", name)
		return false, err
	}

	// TODO: update selector if child labels change

	return false, nil
}

func (rh *requestHandler) reconcilePV() (requeue bool, err error) {
	r := rh.reconciler
	name := string(rh.env.Id()) + "-files"

	var pv *v1.PersistentVolume
	if pv, err = rh.pv(name); err != nil {
		return false, err
	}

	found := &v1.PersistentVolume{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: name}, found)
	if err != nil && errors.IsNotFound(err) {
		rh.logger.Info("Creating PV", "Namespace", pv.Namespace, "Name", pv.Name)
		err = r.client.Create(context.TODO(), pv)
		if err != nil {
			rh.logger.Error(err, "Failed to create PV", "Namespace", pv.Namespace, "Name", pv.Name)
			return false, err
		}
		return true, nil
	}
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile PV", "Name", name)
		return false, err
	}

	// Verify that PV matches expected Spec
	if pvNeedsUpdate(found, pv) {
		rh.logger.Info("Updating PV", "Namespace", found.Namespace, "Name", found.Name)

		found.Spec.PersistentVolumeSource.CSI.VolumeHandle = pv.Spec.PersistentVolumeSource.CSI.VolumeHandle

		err = r.client.Update(context.TODO(), found)
		if err != nil {
			rh.logger.Error(err, "Failed to update PV", "Namespace", found.Namespace, "Name", found.Name)
			return false, err
		}
		return true, nil
	}

	return false, nil
}

func (rh *requestHandler) finalizePV() (requeue bool, err error) {
	r := rh.reconciler
	name := string(rh.env.Id()) + "-files"

	found := &v1.PersistentVolume{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: name}, found); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if found.DeletionTimestamp == nil {
		rh.logger.Info("Deleting PV", "pv name", found.Name)
		return true, r.client.Delete(context.TODO(), found)
	}
	return false, nil
}

func (rh *requestHandler) reconcilePVC() (requeue bool, err error) {
	r := rh.reconciler
	name := string(rh.env.Id()) + "-files"

	pvc := rh.pvc(name)

	found := &v1.PersistentVolumeClaim{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: rh.namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		rh.logger.Info("Creating PVC", "Namespace", pvc.Namespace, "Name", pvc.Name)
		err = r.client.Create(context.TODO(), pvc)
		if err != nil {
			rh.logger.Error(err, "Failed to create PVC", "Namespace", pvc.Namespace, "Name", pvc.Name)
			return false, err
		}
		return true, nil
	}
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile PVC", "Namespace", rh.namespace, "Name", name)
		return false, err
	}

	// PVCs can't be Updated

	return false, nil
}

func pvNeedsUpdate(found, pv *v1.PersistentVolume) bool {
	csi := found.Spec.PersistentVolumeSource.CSI
	if csi != nil && csi.VolumeHandle != pv.Spec.PersistentVolumeSource.CSI.VolumeHandle {
		return true
	}

	return false
}
