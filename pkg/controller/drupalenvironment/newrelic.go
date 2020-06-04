package drupalenvironment

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"text/template"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const newRelicLicenseSecretKey = "license"

const newRelicConfTemplate = `
extension = "newrelic.so"

[newrelic]
newrelic.license = "{{ .License }}"
newrelic.logfile = "/var/log/newrelic/php_agent.log"
newrelic.appname = "{{ .AppName }}"
newrelic.daemon.address = "{{ .Address }}"
newrelic.daemon.dont_launch = 3 ; Never start the New Relic daemon in this container (there's a Deployment for that)
`

var tmplNewRelicConf = template.Must(template.New("tmplNewRelicConf").Parse(newRelicConfTemplate))

func (rh *requestHandler) newRelicConf() (conf string, err error) {
	daemonAddr := os.Getenv("NEWRELIC_DAEMON_ADDR")
	if daemonAddr == "" {
		return "", fmt.Errorf("NEWRELIC_DAEMON_ADDR not set")
	}

	newRelicAppName := rh.env.Spec.Phpfpm.NewRelicAppName
	if newRelicAppName == "" {
		newRelicAppName = fmt.Sprintf("%v - %v", rh.app.Name, rh.env.Name)
	}

	// Get license from referenced Secret
	secret := &v1.Secret{}
	key := types.NamespacedName{Namespace: rh.env.Namespace, Name: rh.env.Spec.Phpfpm.NewRelicSecret}
	if err = rh.reconciler.client.Get(context.TODO(), key, secret); err != nil {
		return "", err
	}

	var license string
	if secret.Data != nil {
		license = string(secret.Data[newRelicLicenseSecretKey])
	}
	if secret.Data == nil || license == "" {
		return "", fmt.Errorf("license key not specified in Secret")
	}

	// Template the config file
	values := struct {
		License, AppName, Address string
	}{
		License: license,
		AppName: newRelicAppName,
		Address: daemonAddr,
	}

	var buf bytes.Buffer
	if err = tmplNewRelicConf.Execute(&buf, values); err == nil {
		conf = buf.String()
	}
	return
}
