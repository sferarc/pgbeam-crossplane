package controller

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	pgbeam "github.com/pgbeam/pgbeam-go"
	"github.com/pgbeam/provider-pgbeam/apis/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	errNotCustomDomain     = "managed resource is not a CustomDomain"
	errCustomDomainCreate  = "cannot create CustomDomain"
	errCustomDomainDelete  = "cannot delete CustomDomain"
	errCustomDomainObserve = "cannot observe CustomDomain"
	errCustomDomainUpdate  = "CustomDomain is fully immutable and cannot be updated; delete and recreate"
)

func SetupCustomDomain(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.CustomDomainGroupKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.CustomDomain{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.CustomDomainGroupVersionKind),
			managed.WithExternalConnecter(&customDomainConnecter{
				connector: connector{
					kube:  mgr.GetClient(),
					usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &v1alpha1.ProviderConfigUsage{}),
				},
			}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		))
}

type customDomainConnecter struct{ connector }

func (c *customDomainConnecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cl, err := c.getClient(ctx, mg)
	if err != nil {
		return nil, err
	}
	return &customDomainExternal{client: cl}, nil
}

type customDomainExternal struct{ client *pgbeam.Client }

func (e *customDomainExternal) Disconnect(_ context.Context) error { return nil }

func (e *customDomainExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.CustomDomain)
	if !ok {
		return managed.ExternalObservation{}, fmt.Errorf(errNotCustomDomain)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	domain, err := e.client.Domains.Get(ctx, cr.Spec.ForProvider.ProjectID, externalName)
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", errCustomDomainObserve, err)
	}

	cr.Status.AtProvider = customDomainObservation(domain)
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (e *customDomainExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.CustomDomain)
	if !ok {
		return managed.ExternalCreation{}, fmt.Errorf(errNotCustomDomain)
	}

	fp := cr.Spec.ForProvider
	domain, err := e.client.Domains.Create(ctx, fp.ProjectID, pgbeam.CreateCustomDomainRequest{Domain: fp.Domain})
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", errCustomDomainCreate, err)
	}

	meta.SetExternalName(cr, domain.ID)
	cr.Status.AtProvider = customDomainObservation(domain)

	connDetails := managed.ConnectionDetails{}
	if domain.DNSVerificationToken != "" {
		connDetails["dnsVerificationToken"] = []byte(domain.DNSVerificationToken)
	}
	return managed.ExternalCreation{ConnectionDetails: connDetails}, nil
}

func (e *customDomainExternal) Update(_ context.Context, _ resource.Managed) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, fmt.Errorf(errCustomDomainUpdate)
}

func (e *customDomainExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.CustomDomain)
	if !ok {
		return managed.ExternalDelete{}, fmt.Errorf(errNotCustomDomain)
	}
	if err := e.client.Domains.Delete(ctx, cr.Spec.ForProvider.ProjectID, meta.GetExternalName(cr)); err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, fmt.Errorf("%s: %w", errCustomDomainDelete, err)
	}
	return managed.ExternalDelete{}, nil
}

func customDomainObservation(d *pgbeam.CustomDomain) v1alpha1.CustomDomainAtProvider {
	obs := v1alpha1.CustomDomainAtProvider{
		ID: d.ID, Verified: d.Verified, DNSVerificationToken: d.DNSVerificationToken,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	if d.VerifiedAt != nil {
		obs.VerifiedAt = d.VerifiedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if d.TLSCertExpiry != nil {
		obs.TLSCertExpiry = d.TLSCertExpiry.Format("2006-01-02T15:04:05Z07:00")
	}
	if d.DNSInstructions != nil {
		obs.DNSInstructions = &v1alpha1.DNSInstructionsObservation{
			CNAMEHost: d.DNSInstructions.CNAMEHost, CNAMETarget: d.DNSInstructions.CNAMETarget,
			TXTHost: d.DNSInstructions.TXTHost, TXTValue: d.DNSInstructions.TXTValue,
			ACMECNAMEHost: d.DNSInstructions.ACMECNAMEHost, ACMECNAMETarget: d.DNSInstructions.ACMECNAMETarget,
		}
	}
	return obs
}
