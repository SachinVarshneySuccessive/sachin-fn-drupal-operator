package site

import (
	fn "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
)

// SiteSynced indicates if a site has become healthy and started to run.
func SiteSynced(site *fn.Site) bool {
	return site.Status.Status == fn.SiteSyncedStatus
}

// SiteSyncing indicates if a site is being processed.
func SiteSyncing(site *fn.Site) bool {
	return site.Status.Status == fn.SiteSyncingStatus
}

// DomainSynced indicates if a domain is synced.
func DomainSynced(site *fn.Site) bool {
	return site.Status.Domains == fn.DomainsSyncedStatus
}

// DomainSynced indicates if site domain is being updated.
func DomainsUpdating(site *fn.Site) bool {
	return site.Status.Domains == fn.DomainsUpdatingStatus
}
