package site

import (
	"context"
	"reflect"

	net "istio.io/api/networking/v1alpha3"
	netv1a3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	extv1b1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/acquia/fn-drupal-operator/pkg/common"
	"github.com/acquia/fn-drupal-operator/pkg/controller/drupalenvironment"
)

func (rh *requestHandler) reconcileIngress() (bool, error) {
	if common.IsIstioEnabled() {
		rh.logger.V(1).Info("Reconciling Istio Ingress Objects")
		return rh.reconcileIstioIngress()
	}
	rh.logger.V(1).Info("Reconciling Kubernetes Ingress Objects")
	return rh.reconcileIngressResource()
}

func (rh *requestHandler) reconcileIstioIngress() (bool, error) {
	r := rh.reconciler

	vs := &netv1a3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rh.site.Name,
			Namespace: rh.site.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, vs, func() error {
		desired := rh.virtualService()

		if vs.CreationTimestamp.IsZero() {
			// Create
			_, _ = common.LinkToOwner(rh.site, vs, rh.reconciler.scheme)
		}

		// Create or Update
		vs.Labels = desired.Labels
		desired.Spec.DeepCopyInto(&vs.Spec)

		return nil
	})
	if err != nil || op == controllerutil.OperationResultNone {
		return false, err
	}
	rh.logger.Info("Reconciled VirtualService", "operation", op)
	return true, nil
}

func (rh *requestHandler) reconcileIngressResource() (bool, error) {
	r := rh.reconciler

	ing := &extv1b1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rh.site.Name,
			Namespace: rh.site.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, ing, func() error {
		desired := rh.ingress()

		if ing.CreationTimestamp.IsZero() {
			// Create
			desired.Spec.DeepCopyInto(&ing.Spec)
			ing.Annotations = desired.Annotations
			ing.Labels = common.MergeLabels(ing.Labels, desired.Labels)
			_, _ = common.LinkToOwner(rh.site, ing, rh.reconciler.scheme)
			return nil
		}

		// Update
		rules, tls, ingAnnotations := ing.Spec.Rules, ing.Spec.TLS, ing.ObjectMeta.Annotations
		desiredRules, desiredTLS := rh.site.IngressRules(), rh.site.IngressTLS()
		if !reflect.DeepEqual(rules, desiredRules) {
			rh.logger.V(1).Info("Ingress rules out of date. Updating...")
			ing.Spec.Rules = desiredRules
		}
		if !reflect.DeepEqual(tls, desiredTLS) {
			rh.logger.V(1).Info("Ingress tls out of date. Updating...")
			ing.Spec.TLS = desiredTLS
		}

		for anno := range desired.Annotations {
			if ingAnnotations[anno] != desired.Annotations[anno] {
				rh.logger.V(1).Info("Ingress annotation out of date. Updating...")
				ing.ObjectMeta.Annotations[anno] = desired.Annotations[anno]
			}
		}

		return nil
	})
	if err != nil || op == controllerutil.OperationResultNone {
		return false, err
	}
	rh.logger.Info("Reconciled Ingress", "operation", op)
	return true, nil
}

func (rh *requestHandler) virtualService() *netv1a3.VirtualService {
	vs := &netv1a3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rh.site.Name,
			Namespace: rh.site.Namespace,
			Labels:    rh.site.ChildLabels(),
		},
		Spec: net.VirtualService{
			Hosts:    rh.site.Spec.Domains,
			Gateways: []string{rh.site.Spec.IngressClass},
			Http: []*net.HTTPRoute{{
				Route: []*net.HTTPRouteDestination{{
					Destination: &net.Destination{
						Host: drupalenvironment.DrupalServiceName,
						Port: &net.PortSelector{
							Number: 80,
						},
					},
				}},
			}},
		},
	}
	return vs
}

func (rh *requestHandler) ingress() *extv1b1.Ingress {
	targetName := rh.site.Name
	targetNamespace := rh.site.Namespace
	ing := &extv1b1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        targetName,
			Namespace:   targetNamespace,
			Labels:      rh.site.ChildLabels(),
			Annotations: rh.desiredIngAnnotations(),
		},
		Spec: extv1b1.IngressSpec{
			Rules: rh.site.IngressRules(),
			TLS:   rh.site.IngressTLS(),
		},
	}
	return ing
}

func (rh *requestHandler) desiredIngAnnotations() map[string]string {
	return map[string]string{
		"certmanager.k8s.io/cluster-issuer": rh.site.IngressCertIssuer(),
		"kubernetes.io/ingress.class":       rh.site.IngressClass(),
	}
}
