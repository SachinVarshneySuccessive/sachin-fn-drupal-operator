package drupalenvironment

import (
	"context"
	"fmt"

	"github.com/acquia/fn-ssh-proxy/pkg/sshtunnel"
	"github.com/argoproj/argo-rollouts/utils/defaults"
	"github.com/go-test/deep"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/acquia/fn-drupal-operator/pkg/common"
	"github.com/acquia/fn-drupal-operator/pkg/customercontainer"
)

const (
	sshdServiceName    = "sshd"
	sshdDeploymentName = "sshd"
)

func (rh *requestHandler) reconcileSSHDService(username string) (result reconcile.Result, err error) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: rh.namespace, Name: sshdServiceName},
	}
	op, err := controllerutil.CreateOrUpdate(context.TODO(), rh.reconciler.client, svc, func() error {
		desired := rh.sshdServiceSpec(username)

		if svc.CreationTimestamp.IsZero() {
			// Create
			desired.DeepCopyInto(&svc.Spec)
			rh.associateResourceWithController(svc)
		} else {
			// Update
			svc.Spec.Ports = desired.Ports
		}
		// Both Create and Update
		svc.Labels = common.MergeLabels(svc.Labels, rh.env.ChildLabels())
		svc.Labels = common.MergeLabels(svc.Labels, sshdAppLabels(username))
		return nil
	})
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile SSH Service")
		return
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Reconciled SSH Service", "op", op)
		result.Requeue = true
	}

	return
}

func (rh *requestHandler) reconcileSSHDDeployment(username string) (result reconcile.Result, err error) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: rh.namespace, Name: sshdDeploymentName},
	}
	op, err := controllerutil.CreateOrUpdate(context.TODO(), rh.reconciler.client, dep, func() error {
		desired := rh.sshdDeploymentSpec(username)

		if dep.CreationTimestamp.IsZero() {
			// Create
			rh.associateResourceWithController(dep)
			dep.Labels = rh.env.ChildLabels()
		}

		if diff := deep.Equal(dep.Spec, desired); diff != nil {
			rh.logger.Info("SSHD Deployment Spec needs update", "current != desired", diff)
			dep.Spec = desired
		}

		dep.Labels = common.MergeLabels(dep.Labels, sshdAppLabels(username))
		return nil
	})
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile SSH Deployment")
		return
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Reconciled SSH Deployment", "op", op)
		result.Requeue = true
	}

	return
}

func (rh *requestHandler) reconcileSSHDAccessControls() (result reconcile.Result, err error) {
	ctx := context.TODO()

	// Create/Update ServiceAccount
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Namespace: rh.namespace, Name: sshdDeploymentName},
	}

	var op controllerutil.OperationResult
	op, err = controllerutil.CreateOrUpdate(ctx, rh.reconciler.client, sa, func() error {
		if sa.CreationTimestamp.IsZero() {
			rh.associateResourceWithController(sa)
		}
		sa.Labels = common.MergeLabels(sa.Labels, rh.env.ChildLabels())
		return nil
	})
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile SSHD ServiceAccount")
		return
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Reconciled SSH ServiceAccount", "op", op)
		result.Requeue = true
		return
	}

	// Create/Update ClusterRoleBinding
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: rh.clusterRoleBindingName()},
	}

	op, err = controllerutil.CreateOrUpdate(ctx, rh.reconciler.client, crb, func() error {
		crb.Labels = common.MergeLabels(crb.Labels, rh.env.ChildLabels())
		crb.Subjects = []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      sshdDeploymentName,
			Namespace: rh.namespace,
		}}
		crb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "sshd",
		}
		return nil
	})
	if err != nil {
		rh.logger.Error(err, "Failed to reconcile SSHD ClusterRoleBinding")
		return
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Reconciled SSH ClusterRoleBinding", "op", op)
		result.Requeue = true
	}
	return
}

func (rh *requestHandler) finalizeSSHDAccessControls() (requeue bool, err error) {
	ctx := context.TODO()
	crb := &rbacv1.ClusterRoleBinding{}
	if err = rh.reconciler.client.Get(ctx, types.NamespacedName{Name: rh.clusterRoleBindingName()}, crb); err != nil {
		if errors.IsNotFound(err) {
			// Already deleted
			return false, nil
		} else {
			return
		}
	}

	if err = rh.reconciler.client.Delete(ctx, crb); err != nil {
		requeue = true
	}
	return
}

func (rh *requestHandler) clusterRoleBindingName() string {
	return fmt.Sprintf("%v-%v", sshdDeploymentName, rh.namespace)
}

func (rh *requestHandler) getSSHUsername() (username string, err error) {
	list := &v1.ConfigMapList{}
	// TODO: use `client.HasLabels()` as well, once we reach controller-runtime v0.5.0
	if err = rh.reconciler.client.List(context.TODO(), list, client.InNamespace(rh.namespace)); err != nil {
		return
	}

	for _, cm := range list.Items {
		if cm.Labels[sshtunnel.LabelSshUser] != "" {
			if username != "" {
				return "", fmt.Errorf("found multiple ConfigMaps labelled with " + sshtunnel.LabelSshUser)
			}
			username = cm.Labels[sshtunnel.LabelSshUser]
		}
	}

	if username != "" {
		return username, nil
	}
	return "", fmt.Errorf("authorized keys ConfigMap not found")
}

func (rh *requestHandler) sshdServiceSpec(username string) v1.ServiceSpec {
	return v1.ServiceSpec{
		Ports: []v1.ServicePort{{
			Name:       "ssh",
			Port:       22,
			TargetPort: intstr.FromString("ssh"),
			Protocol:   "TCP",
		}},
		Selector: sshdAppLabels(username),
	}
}

func (rh *requestHandler) sshdDeploymentSpec(username string) appsv1.DeploymentSpec {
	appLabels := sshdAppLabels(username)
	template := rh.drupalPodTemplate()
	replicas := int32(1)
	terminationGracePeriod := int64(v1.DefaultTerminationGracePeriodSeconds)
	historyLimit := defaults.DefaultRevisionHistoryLimit
	progressDeadline := defaults.DefaultProgressDeadlineSeconds

	// Find php-fpm Container
	var container *v1.Container
	for _, c := range template.Spec.Containers {
		if c.Name == phpFpmContainerName {
			container = &c
			break
		}
	}
	if container == nil {
		// This will only happen if we make an invalid code change, so panic
		panic(fmt.Errorf("no PHP-FPM container in Drupal Pod Template"))
	}

	image := customercontainer.ImageName(rh.app, rh.env)

	container.Name = "sshd"
	container.Image = image
	container.Command = []string{"/sshd-entry.sh"}
	container.Args = []string{"/usr/sbin/sshd", "-D", "-e", "-f", "/etc/ssh/sshd_config"}
	container.Env = append(container.Env, v1.EnvVar{
		Name:  "CUSTOMER_USERNAME",
		Value: username,
	})

	root := int64(0)
	container.SecurityContext = &v1.SecurityContext{RunAsUser: &root} // TODO: don't run as root! https://backlog.acquia.com/browse/NW-1258

	container.Ports = []v1.ContainerPort{{
		Name:          "ssh",
		ContainerPort: 22,
		Protocol:      "TCP",
	}}

	container.Resources = v1.ResourceRequirements{
		// Define only limits, to get "Guaranteed" QoS
		Limits: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("500m"),
			v1.ResourceMemory: resource.MustParse("600Mi"), // 512 MiB PHP CLI limit, plus buffer for opcache, etc.
		},
	}

	template.Labels = common.MergeLabels(rh.env.ChildLabels(), appLabels)
	template.Spec.ServiceAccountName = sshdDeploymentName
	template.Spec.DeprecatedServiceAccount = sshdDeploymentName // Need to set to avoid update loops
	template.Spec.Containers = []v1.Container{*container}
	template.Spec.RestartPolicy = v1.RestartPolicyAlways
	template.Spec.DNSPolicy = v1.DNSClusterFirst
	template.Spec.SecurityContext = &v1.PodSecurityContext{}
	template.Spec.SchedulerName = v1.DefaultSchedulerName
	template.Spec.TerminationGracePeriodSeconds = &terminationGracePeriod

	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: appLabels,
		},
		Template:                template,
		Strategy:                common.DefaultDeploymentStrategy(),
		RevisionHistoryLimit:    &historyLimit,
		ProgressDeadlineSeconds: &progressDeadline,
	}
}

func sshdAppLabels(username string) map[string]string {
	return map[string]string{
		"app":                  "sshd",
		sshtunnel.LabelSshUser: username,
	}
}
