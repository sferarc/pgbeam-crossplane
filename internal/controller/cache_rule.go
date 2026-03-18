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
	errNotCacheRule     = "managed resource is not a CacheRule"
	errCacheRuleCreate  = "cannot create CacheRule"
	errCacheRuleUpdate  = "cannot update CacheRule"
	errCacheRuleDelete  = "cannot soft-delete CacheRule"
	errCacheRuleObserve = "cannot observe CacheRule"
)

func SetupCacheRule(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.CacheRuleGroupKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.CacheRule{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.CacheRuleGroupVersionKind),
			managed.WithExternalConnecter(&cacheRuleConnecter{
				connector: connector{
					kube:  mgr.GetClient(),
					usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &v1alpha1.ProviderConfigUsage{}),
				},
			}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		))
}

type cacheRuleConnecter struct{ connector }

func (c *cacheRuleConnecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cl, err := c.getClient(ctx, mg)
	if err != nil {
		return nil, err
	}
	return &cacheRuleExternal{client: cl}, nil
}

type cacheRuleExternal struct{ client *pgbeam.Client }

func (e *cacheRuleExternal) Disconnect(_ context.Context) error { return nil }

func (e *cacheRuleExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.CacheRule)
	if !ok {
		return managed.ExternalObservation{}, fmt.Errorf(errNotCacheRule)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	fp := cr.Spec.ForProvider
	rule, err := e.client.CacheRules.Get(ctx, fp.ProjectID, fp.DatabaseID, fp.QueryHash)
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", errCacheRuleObserve, err)
	}

	cr.Status.AtProvider = cacheRuleObservation(rule)
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isCacheRuleUpToDate(fp, rule),
	}, nil
}

func (e *cacheRuleExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.CacheRule)
	if !ok {
		return managed.ExternalCreation{}, fmt.Errorf(errNotCacheRule)
	}

	fp := cr.Spec.ForProvider
	rule, err := e.client.CacheRules.Update(ctx, fp.ProjectID, fp.DatabaseID, fp.QueryHash, pgbeam.UpdateCacheRuleRequest{
		CacheEnabled: fp.CacheEnabled, CacheTTLSeconds: fp.CacheTTLSeconds, CacheSWRSeconds: fp.CacheSWRSeconds,
	})
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", errCacheRuleCreate, err)
	}

	compositeID := fmt.Sprintf("%s/%s/%s", fp.ProjectID, fp.DatabaseID, fp.QueryHash)
	meta.SetExternalName(cr, compositeID)
	cr.Status.AtProvider = cacheRuleObservation(rule)

	return managed.ExternalCreation{}, nil
}

func (e *cacheRuleExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.CacheRule)
	if !ok {
		return managed.ExternalUpdate{}, fmt.Errorf(errNotCacheRule)
	}

	fp := cr.Spec.ForProvider
	if _, err := e.client.CacheRules.Update(ctx, fp.ProjectID, fp.DatabaseID, fp.QueryHash, pgbeam.UpdateCacheRuleRequest{
		CacheEnabled: fp.CacheEnabled, CacheTTLSeconds: fp.CacheTTLSeconds, CacheSWRSeconds: fp.CacheSWRSeconds,
	}); err != nil {
		return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", errCacheRuleUpdate, err)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *cacheRuleExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.CacheRule)
	if !ok {
		return managed.ExternalDelete{}, fmt.Errorf(errNotCacheRule)
	}

	fp := cr.Spec.ForProvider
	// Soft delete: disable caching for this query shape.
	if err := e.client.CacheRules.Disable(ctx, fp.ProjectID, fp.DatabaseID, fp.QueryHash); err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, fmt.Errorf("%s: %w", errCacheRuleDelete, err)
	}
	return managed.ExternalDelete{}, nil
}

func cacheRuleObservation(r *pgbeam.CacheRule) v1alpha1.CacheRuleAtProvider {
	return v1alpha1.CacheRuleAtProvider{
		NormalizedSQL: r.NormalizedSQL, QueryType: r.QueryType,
		CallCount: r.CallCount, AvgLatencyMs: r.AvgLatencyMs, P95LatencyMs: r.P95LatencyMs,
		AvgResponseBytes: r.AvgResponseBytes, StabilityRate: r.StabilityRate,
		Recommendation: r.Recommendation, FirstSeenAt: r.FirstSeenAt, LastSeenAt: r.LastSeenAt,
	}
}

func isCacheRuleUpToDate(fp v1alpha1.CacheRuleForProvider, r *pgbeam.CacheRule) bool {
	if fp.CacheEnabled != r.CacheEnabled {
		return false
	}
	if (fp.CacheTTLSeconds == nil) != (r.CacheTTLSeconds == nil) {
		return false
	}
	if fp.CacheTTLSeconds != nil && r.CacheTTLSeconds != nil && *fp.CacheTTLSeconds != *r.CacheTTLSeconds {
		return false
	}
	if (fp.CacheSWRSeconds == nil) != (r.CacheSWRSeconds == nil) {
		return false
	}
	if fp.CacheSWRSeconds != nil && r.CacheSWRSeconds != nil && *fp.CacheSWRSeconds != *r.CacheSWRSeconds {
		return false
	}
	return true
}
