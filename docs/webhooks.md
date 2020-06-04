# Webhooks

Webhooks (or **admission controllers**) are a relatively simple concept in
theory.
[Here](https://kubernetes.io/blog/2019/03/21/a-guide-to-kubernetes-admission-controllers/)
you can find a detailed description of how the admission process works, but this
diagram shows the process in a nutshell: ![webhooks](https://d33wubrfki0l68.cloudfront.net/af21ecd38ec67b3d81c1b762221b4ac777fcf02d/7c60e/images/blog/2019-03-21-a-guide-to-kubernetes-admission-controllers/admission-controller-phases.png)

In other words, webhooks provide a mechanism to hook into the kubernets API
*before* a resource is excepted or modified in etcd.  This allows us to do
things that are not possible with a controller alone.

## Validation vs Mutation
A mutating webhook can be a convenient way to add defaults to your objects.
This means that when you make a request, before it hits etcd, certain fields may
be set to a default value.  by the time your controller gets ahold of a
defaulted object, these values have already been set.  We have been mimicing
this behavior in the controller itself.  Some simplification would surely be
gained by implementing these defaulting webhooks, but overall, no drastically
new functionality would be gained.  Mutating webhooks can also be used to
provide more advanced behavior, like adding a sidecar to every pod (like istio)
but that sort of behavior is unlikely to be needed for us.

Validating webhooks on the other hand are very important for our CRDs.  They
provide *semantic* validation, whereas the CRD itself only provides *syntactic*
validation.  Without a validating webhook, a request that we know will fail but
is syntactically valid would make it to etcd and result in error logs in the
operator, but the "user" would be unaware of the failure.  Once you add a
validating webhook into the mix, the actual API request will fail with our error
message, bringing the error to where it needs to be: in the response of the bad
request, not buried in logs.

## Implementation Ecosystem

This is where webhooks get complex.  The `operator-sdk`, the framework we use to
build our operators has not really worked out how they want to support webhooks.
There are several interfaces, several examples of out of date code, and many
different ways to build, implement, register, change settings of, and deploy
webhooks.  And almost none of them are documented.  But here's how the current
implementation works roughly.

There are basically six parts to a webhook (that I can tell). 
1. Webhook (struct) creation
1. Webhook Server creation/settings
1. Webhook registration with server
1. Webhook Server registration with manager
1. Webhook implementation (the code you actually care about), and finally
1. Registration with the k8s API

There are many many ways to implement each of those parts, but by crawling
through the `controller-runtime` code, I believe the best way boils down to
only a few parts:
1. [Webhook
   implementation](#webhook-implementation)
1. [Manager registration](#manager-registration)
1. [Server settings](#server-settings)
1. [API registration](#api-registration)


### Webhook Implementation

Again, there are multiple ways to do this.  You will find examples of the
"Handle" interface, like shown in [controller-runtime
examples](https://github.com/kubernetes-sigs/controller-runtime/blob/master/examples/builtins/validatingwebhook.go)
and in the [kubebuilder
book](https://book-v1.book.kubebuilder.io/beyond_basics/sample_webhook.html).
Both of these example in my opinion are out-dated and overly complex.  Instead,
controller-runtime provides a [`Validator`
interface](https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/webhook/admission/validator.go#L28)
that itself implements the `Handle` interface above.  By implementing this
interface, we end up with a very simple implementation of all the golang
components of a webhook.  

```golang
import "sigs.k8s.io/controller-runtime/pkg/webhook"

var _ webhook.Validator = &Resource{}

func (r *Resource) ValidateCreate() error {
    // Validate Create
	return nil
}

func (r *Resource) ValidateUpdate(old runtime.Object) error {
    // Validate Update
	return nil
}

func (r *Resource) ValidateDelete() error {
    // Validate Delete
	return nil
}
```

A similar interface exists for a Defaulting webhook.

```golang
import "sigs.k8s.io/controller-runtime/pkg/webhook"

var _ webhook.Defaulter = &Resource{}

func (r *Resource) Default() {

}
```

If you choose to go this route, the next step becomes very simple as well.

### Manager Registration

There is a chain of registration at play here.  `Handlers` get registered to
`Webhooks` get registered to `WebhookServers` get registered to the `Manager`.
Laid out clearly:
```
//  -> == Registered to 
Handers -> Webhooks -> WebhookServer(s) -> Manager
```

There is documentation on the `Manager`, but basically it is the main
orchestrator of an operator, responsible for running all `Controllers` and
`WebhookServers`.  Luckily the controller side of the house was all generated
for us.  

When you use the Validator and Defaulter interface like above, all the
registration can be done in the already-generated `Add` function in the
controller.  This does have the effect of mixing contoller logic and webhook
logic just a tiny bit, but I hardly think it matters. 

```golang
func Add(mgr manager.Manager) error {
	err := builder.
		WebhookManagedBy(mgr).
		For(&Resource{}).
		Complete()
	if err != nil {
		log.Error(err, "could not create resource webhook")
		return err
	}
	return add(mgr, newReconciler(mgr))
}

```

This code does the following:
1. Since `Resource` implements the `Validator` and `Defaulter` interfaces, a
   `Handler` already exists for them.  (both of these implement `Handler`).  
1. a `Webhook` is created `For` `Resource`.
1. Internally, this is registered to the default `WebhookServer` which is
   already registered to the `Manager`.  This server can be retrieved with
   `mgr.GetWebhookServer()`. This is done by `WebhookManagedBy(mgr)` above.
1. the final `return add(...)` was part of the autogenerated controller code.

This means, that we've hooked into our Controller creating and registration to
simultaneously create and register our Webhooks to a single WebhookServer.  This
means in our `main` we can now configure our server how we like.


### Server Settings

Now that all our webhooks are registered, we need to change a few settings on
our `WebhookServer` so that things will run correctly.  This can be done with
the following:
```golang
 // Webhook server configuration
 server := mgr.GetWebhookServer()
 server.Port = 8443
```

Other settings can be changed as well, but this is the only one that is
necessary.  By default, the `Port` will be 443, and since we do not run our
operators as root, this binding will fail.  It's an odd default, but again, this
stuff isn't well integrated into the operator-sdk currently.

If at this point, you run the operator, you will find that it will crash.  That
is because the `WebhookServer` is now looking for a certificate file at a
certain path.  This must be created and mounted as a secret into the pod.  More
on that [later](#certificates).  You will find the path it is trying to read
from in the operator logs.

### API Registration

Finally, our operator runs, and everything appears to be working!  We go to test
our new webhook by `CREATE`ing a `Resource` and!!!!  Nothing happens. Or nothing
different anyways.  This is because we never told the k8s api to *call* our
webhook.  We never hooked it into the diagram shown at the top of this file.  In
order to do that, we must create a
[`ValidatingWebhookConfiguration`](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#configure-admission-webhooks-on-the-fly).
This is a k8s resource that instructs the apiserver what to call, and how to
call it.  We point it at a `service` on a port and a path.  Once this is
configured correctly, the webhook will fire as expected.  

#### Extra Plumbing

All this requires a little bit of extra plumbing.  I've already mentioned the
`ValidatingWebhookConfiguration` (or `MutatingWebhookConfiguration`) but that
needs a `Service` to point to, and a `Certificate` for tls.  Without going too
much into the weeds of how all this works, you can find all of these pieces in
our [drupal operator deployment files](../deploy/webhook.yaml).

##### Certificates

Certificates can be a little difficult to get right.  There are many options and
scripts out there for how to generate certificates.  In fact, there is an API
for generating a certificate signed by the [k8s api
itself](https://github.com/newrelic/k8s-webhook-cert-manager).  However, I found
the easiest way to do this was to install
[cert-manager](https://github.com/jetstack/cert-manager) on the cluster and use
it to generate a self-signed certificate.  Since all traffic is internal, this
seems sufficient.

The resources I created can be found linked [above](#extra-plumbing), but
essentially you need an `Issuer` which is capable of provisioning self-signed
certs, and a `Certificate` which uses that issuer to create a secret with
`ca.crt`, `tls.crt`, and `tls.key`.  You can then annotate the
`ValidatingWebhookConfiguration` to inject the `caBundle` from this
`Certificate` into the `ValidatingWebhookConfiguration`.  Seperately, mount the
created `Secret` into the operator pod at the required path (from the operator
logs).  I encourage anyone who thinks this sounds complex to look at the yaml
files linked above, as it is far more complex in English!  
