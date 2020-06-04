package drupalenvironment

import (
	"strings"

	"github.com/acquia/fn-drupal-operator/pkg/customercontainer"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconcileApacheConfEnabledConfigMap reconciles the "apache-conf-enabled" ConfigMap for the requested
// DrupalEnvironment. This ConfigMap contains ".conf" files that will be enabled on the Apache container.
func (rh *requestHandler) reconcileApacheConfEnabledConfigMap() (result reconcile.Result, err error) {
	environmentVariables := customercontainer.EnvironmentVariables(rh.env)

	environmentVariableNames := environmentVariableNames(environmentVariables)

	var requeue bool
	requeue, err = rh.reconcileConfigMap("apache-conf-enabled", map[string]string{
		"passenv.conf": "PassEnv " + strings.Join(environmentVariableNames, " "),
	})

	if err != nil || requeue {
		return reconcile.Result{Requeue: requeue}, err
	}

	return
}

func environmentVariableNames(environmentVariables []v1.EnvVar) []string {
	var list []string
	for _, environmentVariable := range environmentVariables {
		list = append(list, environmentVariable.Name)
	}
	return list
}
