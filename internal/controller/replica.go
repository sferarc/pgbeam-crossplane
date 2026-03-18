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
	errNotReplica     = "managed resource is not a Replica"
	errReplicaCreate  = "cannot create Replica"
	errReplicaDelete  = "cannot delete Replica"
	errReplicaObserve = "cannot observe Replica"
	errReplicaUpdate  = "Replica is fully immutable and cannot be updated; delete and recreate"
)

func SetupReplica(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ReplicaGroupKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Replica{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.ReplicaGroupVersionKind),
			managed.WithExternalConnecter(&replicaConnecter{
				connector: connector{
					kube:  mgr.GetClient(),
					usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &v1alpha1.ProviderConfigUsage{}),
				},
			}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		))
}

type replicaConnecter struct{ connector }

func (c *replicaConnecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cl, err := c.getClient(ctx, mg)
	if err != nil {
		return nil, err
	}
	return &replicaExternal{client: cl}, nil
}

type replicaExternal struct{ client *pgbeam.Client }

func (e *replicaExternal) Disconnect(_ context.Context) error { return nil }

func (e *replicaExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Replica)
	if !ok {
		return managed.ExternalObservation{}, fmt.Errorf(errNotReplica)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	replica, err := e.client.Replicas.Get(ctx, cr.Spec.ForProvider.DatabaseID, externalName)
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", errReplicaObserve, err)
	}

	cr.Status.AtProvider = v1alpha1.ReplicaAtProvider{
		ID: replica.ID, CreatedAt: replica.CreatedAt, UpdatedAt: replica.UpdatedAt,
	}
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (e *replicaExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Replica)
	if !ok {
		return managed.ExternalCreation{}, fmt.Errorf(errNotReplica)
	}

	fp := cr.Spec.ForProvider
	replica, err := e.client.Replicas.Create(ctx, fp.DatabaseID, pgbeam.CreateReplicaRequest{
		Host: fp.Host, Port: fp.Port, SSLMode: fp.SSLMode,
	})
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", errReplicaCreate, err)
	}

	meta.SetExternalName(cr, replica.ID)
	cr.Status.AtProvider = v1alpha1.ReplicaAtProvider{
		ID: replica.ID, CreatedAt: replica.CreatedAt, UpdatedAt: replica.UpdatedAt,
	}
	return managed.ExternalCreation{}, nil
}

func (e *replicaExternal) Update(_ context.Context, _ resource.Managed) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, fmt.Errorf(errReplicaUpdate)
}

func (e *replicaExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Replica)
	if !ok {
		return managed.ExternalDelete{}, fmt.Errorf(errNotReplica)
	}
	if err := e.client.Replicas.Delete(ctx, cr.Spec.ForProvider.DatabaseID, meta.GetExternalName(cr)); err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, fmt.Errorf("%s: %w", errReplicaDelete, err)
	}
	return managed.ExternalDelete{}, nil
}
