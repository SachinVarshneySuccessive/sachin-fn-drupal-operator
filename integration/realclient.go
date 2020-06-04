package integration

import (
	"k8s.io/apimachinery/pkg/runtime"
	k8s "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/acquia/fn-drupal-operator/pkg/apis"
)

func NewRealClient() (client.Client, error) {
	scheme := runtime.NewScheme()
	err := apis.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	err = k8s.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	return client.New(cfg, client.Options{Scheme: scheme})
}
