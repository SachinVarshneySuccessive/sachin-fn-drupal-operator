package envconfig

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/common"
)

const envConfigSecretName = "env-config"

var log = logf.Log.WithName("envconfig")

func UpdateDrupalSettingsConfig(c client.Client, scheme *runtime.Scheme, drenv *v1alpha1.DrupalEnvironment,
	site *v1alpha1.Site, settingsInc []byte) (result reconcile.Result, err error) {

	filename := fmt.Sprintf("%s.settings.inc", v1alpha1.DrupalSiteName(site))
	return createOrUpdateEnvConfigEntry(c, scheme, drenv, filename, settingsInc)
}

func UpdateDrupalSitesConfig(c client.Client, scheme *runtime.Scheme, drenv *v1alpha1.DrupalEnvironment, sitesInc []byte,
) (result reconcile.Result, err error) {

	return createOrUpdateEnvConfigEntry(c, scheme, drenv, "sites.inc", sitesInc)
}

func createOrUpdateEnvConfigEntry(c client.Client, scheme *runtime.Scheme, drenv *v1alpha1.DrupalEnvironment, filename string, incFile []byte,
) (result reconcile.Result, err error) {

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: drenv.Namespace, Name: envConfigSecretName},
	}
	logger := log.WithValues("Namespace", secret.Namespace, "Name", secret.Name)

	var op controllerutil.OperationResult
	op, err = controllerutil.CreateOrUpdate(context.TODO(), c, secret, func() error {
		if secret.CreationTimestamp.IsZero() {
			_, _ = common.LinkToOwner(drenv, secret, scheme) // Will always succeed, since it's a new resource
		}
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}

		if incFile == nil {
			// Remove entry
			delete(secret.Data, filename)
		} else {
			// Add/update entry
			secret.Data[filename] = incFile
		}

		secret.Labels = common.MergeLabels(secret.Labels, drenv.ChildLabels())
		return nil
	})
	if err != nil {
		logger.Error(err, "Failed to reconcile Environment Config Secret")
		return
	}
	if op != controllerutil.OperationResultNone {
		logger.Info("Reconciled Environment Config Secret", "op", op)
		result.Requeue = true
	}

	return
}
