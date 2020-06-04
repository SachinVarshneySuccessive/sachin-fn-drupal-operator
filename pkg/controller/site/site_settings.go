package site

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	"github.com/acquia/fn-drupal-operator/pkg/envconfig"
)

// Create Go templates for settings.php code generation
const drupalSettingsTemplate = `<?php

$settings['file_private_path'] = '/shared/private-files';

if (empty($settings['hash_salt'])) {
  $settings['hash_salt'] = '{{ .HashSalt }}';
}

{{ range .Databases -}}
$databases['{{ .Key }}']['default'] = [
  'database' => '{{ .Database }}',
  'username' => '{{ .Username }}',
  'password' => '{{ .Password }}',
  'host' => '{{ .Host }}',
  'port' => '{{ .Port }}',
  'driver' => '{{ .Driver }}',
  'prefix' => '{{ .Prefix }}',
];
{{- end }}

$settings['trusted_host_patterns'] = [
  '^localhost$',
  '^127\.\d{1,3}\.\d{1,3}\.\d{1,3}$',
{{- range .Domains }}
  '^{{ escapeDots . }}$',
{{- end }}
];
`

var tmplDrupalSettings = template.Must(template.New("tmplDrupalSettings").
	Funcs(template.FuncMap{
		"escapeDots": escapeDots,
	}).
	Parse(drupalSettingsTemplate))

type DrupalSiteConfig struct {
	Domains   []string
	Databases []DrupalDBConfig
	HashSalt  string
}

type DrupalDBConfig struct {
	// Key contains the database key used in settings.php $databases, typically 'default'
	Key string

	Database string
	Username string
	Password string
	Host     string
	Port     string
	Driver   string
	Prefix   string
	// Collation string
}

// updateSiteSettings reconciles the site's settings include file held in the "env-config" Secret.
func (rh *requestHandler) updateSiteSettings() (result reconcile.Result, err error) {
	var siteConfig *DrupalSiteConfig
	if siteConfig, err = rh.newSiteConfig(); err != nil {
		if errors.IsNotFound(err) {
			result.RequeueAfter = 10 * time.Second
			err = nil
		}
		return
	}

	// Generate settings include file contents for this Site
	var settingsInc []byte
	if settingsInc, err = rh.generateSettingsInclude(siteConfig); err != nil {
		rh.logger.Error(err, "Failed to generate Drupal settings include")
		return
	}

	// Create/Update the Secret resource
	r := rh.reconciler
	return envconfig.UpdateDrupalSettingsConfig(r.client, r.scheme, rh.env, rh.site, settingsInc)
}

// cleanupSiteSettings removed the site's settings include file held in the "env-config" Secret.
func (rh *requestHandler) cleanupSiteSettings() (result reconcile.Result, err error) {
	r := rh.reconciler

	// Get parent environment
	err = rh.reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: rh.site.Namespace, Name: rh.site.Spec.Environment}, rh.env)
	if err != nil {
		if errors.IsNotFound(err) {
			rh.logger.Info("Failed to get parent environment for site settings cleanup", "Environment", rh.site.Spec.Environment)

			// Can't clean up any further, and likely DrupalEnvironment and Namespace are already being deleted, so just return.
			return result, nil
		}
		return
	}

	rh.logger.Info("Removing Drupal settings from env-config")
	return envconfig.UpdateDrupalSettingsConfig(r.client, r.scheme, rh.env, rh.site, nil)
}

func (rh *requestHandler) newSiteConfig() (config *DrupalSiteConfig, err error) {
	ctx := context.TODO()
	db := &v1alpha1.Database{}

	key := types.NamespacedName{Namespace: rh.site.Namespace, Name: rh.site.Spec.Database}
	if err = rh.reconciler.client.Get(ctx, key, db); err != nil {
		if errors.IsNotFound(err) {
			rh.logger.Error(err, "Site's Database not found")
			return
		} else {
			rh.logger.Error(err, "Failed to get Site's Database")
		}
		return
	}

	var conn v1alpha1.ConnectionConfig
	if conn, err = db.GetConnectionConfig(rh.reconciler.client); err != nil {
		rh.logger.Error(err, "Failed to get DB ConnectionConfig")
		return
	}

	return &DrupalSiteConfig{
		Domains:  rh.site.Spec.Domains,
		HashSalt: "fake garbage", // TODO: https://backlog.acquia.com/browse/NW-130

		Databases: []DrupalDBConfig{{
			Key:      "default",
			Database: conn.Name,
			Username: conn.User,
			Password: conn.Password,
			Host:     conn.Host,
			Port:     strconv.Itoa(conn.Port),
			Driver:   "mysql",
			Prefix:   "",
		}},
	}, nil
}

func (rh *requestHandler) generateSettingsInclude(siteConfig *DrupalSiteConfig) (settingsInc []byte, err error) {
	var buf bytes.Buffer
	if err = tmplDrupalSettings.Execute(&buf, siteConfig); err == nil {
		settingsInc = buf.Bytes()
	}
	return
}

// escapeDots escapes "." characters in the given string, for use in a regex pattern.
func escapeDots(host string) string {
	return strings.ReplaceAll(host, ".", "\\.")
}
