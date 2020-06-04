# Istio support for Drupal Environments

At the present time Istio support is not complete.  There are security and
scalability features outstanding.  However, the basic functionality of running
a drupal site in a service mesh should work.

## Requirements

1. Cluster has `istio-operator` installed with a configured `istio` CR.
    * Verify with `kubectl get istios -n istio-system`
    * If not, this can easily be set up with the
    [backyards-cli](https://github.com/banzaicloud/backyards-cli).  See [documentation](https://github.com/acquia/fn-polaris/blob/master/doc/creating-a-cluster.md) in fn-polaris.
    * make sure you're running istio 1.4+ if you didn't install this yourself.

1. The fn-drupal-operator helm chart must have `istio.enabled=true`  set.

If these steps are taken after installing the operator, make sure to cycle the pod.
If these steps are taken after installing an environment, it is suggested to delete the
environment and reinstall, but you may also attempt to simply cycle the pods.

_NOTE_: cycling the drupal operator after configuring istio, but before
deleting the environment may lead to an interesting state.  The operator may
not be able to communicate with the things it needs to in order to satisfy the
finalizers on the fnresources CRs.  If you get into this state, simply remove
the finalizers off all CRs that refuse to delete.

### Installing a site

When in stalling a site under an Istio configuration, it is **required** to set
the `ingressClass` field and to provide at least one `host`.  `ingressClass`
must be set to the `namespace/name` of the `Gateway` installed prior.  Most
commonly, this will be `istio-system/drupal-gateway`.

Also, remember to set `tls: false`.

## Verifying correct setup

If all goes well, you should see no alarming, continuous errors from the operator logs.  You should see `istio-proxy` sidecars in the operator pod as well as all environment pods.  You should see a `VirtualService` in place of an `Ingress` rule.  To verify end to end that this is working correctly, do the following steps:
```bash
export LOAD_BALANCER=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
export INGRESS_PORT=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].port}')
export DOMAIN=$(kubectl get site wlgore-site -o jsonpath='{.spec.domains[0]}')

curl -I http://$LOAD_BALANCER:$INGRESS_PORT -HHost:$DOMAIN
```
You should see a response code of 200 on the curl.


## TODO
* Istio-enabled sites do not support TLS yet.
* Sidecar configurations need to be installed to prevent services from getting configurations to talk to other customers. Without this, we will run into scale issues with Pilot.
* There are various other security rules the Environment should set up.  These are not yet implemented.  The site runs with the same security status and abilities as it's always had in regards to the network.  It simply runs in the service mesh.
