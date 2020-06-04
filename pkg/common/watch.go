package common

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func WatchOwned(c controller.Controller, ownerType runtime.Object, types []runtime.Object) (err error) {
	for _, t := range types {
		err = c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    ownerType,
		})
		if err != nil {
			return
		}
	}
	return
}
