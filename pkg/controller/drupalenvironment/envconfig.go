package drupalenvironment

import (
	"bytes"
	"context"
	"sort"
	"text/template"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fnv1alpha1 "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/envconfig"
)

// Create Go templates for sites.php code generation
const drupalSitesTemplate = `<?php

{{ range $domain, $sitename := . -}}
$sites['{{ $domain }}'] = '{{ $sitename }}';
{{ end }}
`

var tmplDrupalSites = template.Must(template.New("tmplDrupalSettings").Parse(drupalSitesTemplate))

type Domain string
type SiteName string

type DomainMap map[Domain]SiteName

// reconcileEnvConfigSecret reconciles the Environment-wide settings in the "env-config" Secret for the requested
// DrupalEnvironment. The "env-config" Secret contains Drupal configuration files that sre needed by "sites.php" and
// "settings.php" to properly configure the environment to match Polaris resource state.
func (rh *requestHandler) reconcileEnvConfigSecret() (result reconcile.Result, err error) {
	ctx := context.TODO()

	sites := &fnv1alpha1.SiteList{}
	err = rh.reconciler.client.List(ctx, sites, client.InNamespace(rh.env.Namespace), client.MatchingLabels(rh.env.ChildLabels()))
	if err != nil {
		rh.logger.Error(err, "Failed to list Sites")
		return
	}

	// Sort sites by name to avoid spurious updates due to indefinite ordering
	sort.Slice(sites.Items, func(i, j int) bool {
		return sites.Items[i].Name < sites.Items[j].Name
	})

	domainMap := make(DomainMap)
	for _, site := range sites.Items {
		for _, domain := range site.Spec.Domains {
			domainMap[Domain(domain)] = SiteName(fnv1alpha1.DrupalSiteName(&site))
		}
	}

	var sitesInc []byte
	if sitesInc, err = generateSitesInc(domainMap); err != nil {
		rh.logger.Error(err, "Failed to generate sites.php config")
		return
	}

	r := rh.reconciler
	if result, err = envconfig.UpdateDrupalSitesConfig(r.client, r.scheme, rh.env, sitesInc); err != nil {
		rh.logger.Error(err, "Failed to update sites.php config")
	}
	return
}

// func sortedDomainMap() // TODO

func generateSitesInc(domainMap DomainMap) (sitesInc []byte, err error) {
	var buf bytes.Buffer
	if err = tmplDrupalSites.Execute(&buf, domainMap); err == nil {
		sitesInc = buf.Bytes()
	}
	return
}
