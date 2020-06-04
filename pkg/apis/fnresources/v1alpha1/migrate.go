package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type versionedType interface {
	metav1.Object

	// SpecVersion returns the latest resource Spec version number for this type. Any resources with an earlier version
	// label (or no label) should be processed by doMigrate().
	SpecVersion() string
	migrationNeeded() bool
	doMigrate()
}

func Migrate(t versionedType) bool {
	if !t.migrationNeeded() && t.SpecVersion() == ObjectVersion(t) {
		return false
	}

	if t.migrationNeeded() {
		t.doMigrate()
	}

	setObjectVersion(t, t.SpecVersion())
	return true
}

func ObjectVersion(t versionedType) string {
	return t.GetLabels()[VersionLabel]
}

func setObjectVersion(t versionedType, version string) {
	l := t.GetLabels()
	if l == nil {
		l = map[string]string{}
	}
	l[VersionLabel] = version
	t.SetLabels(l)
}
