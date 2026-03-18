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
	errNotSpendLimit     = "managed resource is not a SpendLimit"
	errSpendLimitCreate  = "cannot create SpendLimit"
	errSpendLimitUpdate  = "cannot update SpendLimit"
	errSpendLimitDelete  = "cannot soft-delete SpendLimit"
	errSpendLimitObserve = "cannot observe SpendLimit"
)

func SetupSpendLimit(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.SpendLimitGroupKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.SpendLimit{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.SpendLimitGroupVersionKind),
			managed.WithExternalConnecter(&spendLimitConnecter{
				connector: connector{
					kube:  mgr.GetClient(),
					usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &v1alpha1.ProviderConfigUsage{}),
				},
			}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		))
}

type spendLimitConnecter struct{ connector }

func (c *spendLimitConnecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cl, err := c.getClient(ctx, mg)
	if err != nil {
		return nil, err
	}
	return &spendLimitExternal{client: cl}, nil
}

type spendLimitExternal struct{ client *pgbeam.Client }

func (e *spendLimitExternal) Disconnect(_ context.Context) error { return nil }

func (e *spendLimitExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.SpendLimit)
	if !ok {
		return managed.ExternalObservation{}, fmt.Errorf(errNotSpendLimit)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	plan, err := e.client.Analytics.GetOrganizationPlan(ctx, cr.Spec.ForProvider.OrgID)
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", errSpendLimitObserve, err)
	}

	cr.Status.AtProvider = spendLimitObservation(plan)
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isSpendLimitUpToDate(cr.Spec.ForProvider, plan),
	}, nil
}

func (e *spendLimitExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.SpendLimit)
	if !ok {
		return managed.ExternalCreation{}, fmt.Errorf(errNotSpendLimit)
	}

	fp := cr.Spec.ForProvider
	plan, err := e.client.Analytics.UpdateSpendLimit(ctx, fp.OrgID, pgbeam.UpdateSpendLimitRequest{SpendLimit: fp.SpendLimit})
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", errSpendLimitCreate, err)
	}

	meta.SetExternalName(cr, fp.OrgID)
	cr.Status.AtProvider = spendLimitObservation(plan)
	return managed.ExternalCreation{}, nil
}

func (e *spendLimitExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.SpendLimit)
	if !ok {
		return managed.ExternalUpdate{}, fmt.Errorf(errNotSpendLimit)
	}

	fp := cr.Spec.ForProvider
	if _, err := e.client.Analytics.UpdateSpendLimit(ctx, fp.OrgID, pgbeam.UpdateSpendLimitRequest{SpendLimit: fp.SpendLimit}); err != nil {
		return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", errSpendLimitUpdate, err)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *spendLimitExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.SpendLimit)
	if !ok {
		return managed.ExternalDelete{}, fmt.Errorf(errNotSpendLimit)
	}

	// Soft delete: remove the spend limit by setting it to nil.
	if err := e.client.Analytics.RemoveSpendLimit(ctx, cr.Spec.ForProvider.OrgID); err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, fmt.Errorf("%s: %w", errSpendLimitDelete, err)
	}
	return managed.ExternalDelete{}, nil
}

func spendLimitObservation(p *pgbeam.OrganizationPlan) v1alpha1.SpendLimitAtProvider {
	obs := v1alpha1.SpendLimitAtProvider{Plan: p.Plan}
	if p.BillingProvider != nil {
		obs.BillingProvider = *p.BillingProvider
	}
	if p.SubscriptionStatus != nil {
		obs.SubscriptionStatus = *p.SubscriptionStatus
	}
	if p.CurrentPeriodEnd != nil {
		obs.CurrentPeriodEnd = p.CurrentPeriodEnd.Format("2006-01-02T15:04:05Z07:00")
	}
	obs.Enabled = p.Enabled
	obs.CustomPricing = p.CustomPricing
	obs.Limits = &v1alpha1.PlanLimitsObservation{
		QueriesPerDay: p.Limits.QueriesPerDay, MaxProjects: p.Limits.MaxProjects,
		MaxDatabases: p.Limits.MaxDatabases, MaxConnections: p.Limits.MaxConnections,
		QueriesPerSecond: p.Limits.QueriesPerSecond, BytesPerMonth: p.Limits.BytesPerMonth,
		MaxQueryShapes: p.Limits.MaxQueryShapes, IncludedSeats: p.Limits.IncludedSeats,
	}
	return obs
}

func isSpendLimitUpToDate(fp v1alpha1.SpendLimitForProvider, p *pgbeam.OrganizationPlan) bool {
	if fp.SpendLimit == nil && p.SpendLimit == nil {
		return true
	}
	if fp.SpendLimit == nil || p.SpendLimit == nil {
		return false
	}
	return *fp.SpendLimit == *p.SpendLimit
}
