package v1alpha1

const (
	LabelPrefix        = "fnresources.acquia.io/"
	ApplicationIdLabel = LabelPrefix + "application-id"
	EnvironmentIdLabel = LabelPrefix + "environment-id"
	SiteIdLabel        = LabelPrefix + "site-id"
	DatabaseIdLabel    = LabelPrefix + "database-id"
	CronIdLabel        = LabelPrefix + "cron-id"
	GitRepoLabel       = LabelPrefix + "git-repo"
	GitRefLabel        = LabelPrefix + "git-ref"
	StageLabel         = LabelPrefix + "stage"
	VersionLabel       = LabelPrefix + "version"

	ConfigHashAnnotation = LabelPrefix + "php-apache-config-hash"
)
