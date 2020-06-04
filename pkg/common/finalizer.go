package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasFinalizer checks if a Resource has the given finalizer.
func HasFinalizer(resource metav1.Object, finalizer string) bool {
	current := resource.GetFinalizers()
	for _, f := range current {
		if f == finalizer {
			return true
		}
	}
	return false
}
