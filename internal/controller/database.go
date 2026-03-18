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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errNotDatabase      = "managed resource is not a Database"
	errDatabaseCreate   = "cannot create Database"
	errDatabaseUpdate   = "cannot update Database"
	errDatabaseDelete   = "cannot delete Database"
	errDatabaseObserve  = "cannot observe Database"
	errDatabasePassword = "cannot read database password"
)

func SetupDatabase(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.DatabaseGroupKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Database{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.DatabaseGroupVersionKind),
			managed.WithExternalConnecter(&databaseConnecter{
				connector: connector{
					kube:  mgr.GetClient(),
					usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &v1alpha1.ProviderConfigUsage{}),
				},
			}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		))
}

type databaseConnecter struct{ connector }

func (c *databaseConnecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cl, err := c.getClient(ctx, mg)
	if err != nil {
		return nil, err
	}
	return &databaseExternal{client: cl, kube: c.kube, connector: &c.connector}, nil
}

type databaseExternal struct {
	client    *pgbeam.Client
	kube      client.Client
	connector *connector
}

func (e *databaseExternal) Disconnect(_ context.Context) error { return nil }

func (e *databaseExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Database)
	if !ok {
		return managed.ExternalObservation{}, fmt.Errorf(errNotDatabase)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	db, err := e.client.Databases.Get(ctx, cr.Spec.ForProvider.ProjectID, externalName)
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", errDatabaseObserve, err)
	}

	cr.Status.AtProvider = databaseObservation(db)
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isDatabaseUpToDate(cr.Spec.ForProvider, db),
	}, nil
}

func (e *databaseExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Database)
	if !ok {
		return managed.ExternalCreation{}, fmt.Errorf(errNotDatabase)
	}

	fp := cr.Spec.ForProvider
	password, err := e.connector.getSecretValue(ctx, fp.PasswordSecretRef)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", errDatabasePassword, err)
	}

	req := pgbeam.CreateDatabaseRequest{
		Host: fp.Host, Port: fp.Port, Name: fp.Name,
		Username: fp.Username, Password: password,
		SSLMode: fp.SSLMode, Role: fp.Role,
	}
	if fp.PoolRegion != nil {
		req.PoolRegion = fp.PoolRegion
	}
	if fp.CacheConfig != nil {
		req.CacheConfig = &pgbeam.CacheConfig{
			Enabled: fp.CacheConfig.Enabled, TTLSeconds: fp.CacheConfig.TTLSeconds,
			MaxEntries: fp.CacheConfig.MaxEntries, SWRSeconds: fp.CacheConfig.SWRSeconds,
		}
	}
	if fp.PoolConfig != nil {
		req.PoolConfig = &pgbeam.PoolConfig{
			PoolSize: fp.PoolConfig.PoolSize, MinPoolSize: fp.PoolConfig.MinPoolSize,
			PoolMode: fp.PoolConfig.PoolMode,
		}
	}

	db, err := e.client.Databases.Create(ctx, fp.ProjectID, req)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", errDatabaseCreate, err)
	}

	meta.SetExternalName(cr, db.ID)
	cr.Status.AtProvider = databaseObservation(db)

	connDetails := managed.ConnectionDetails{}
	if db.ConnectionString != nil {
		connDetails["connectionString"] = []byte(*db.ConnectionString)
	}
	return managed.ExternalCreation{ConnectionDetails: connDetails}, nil
}

func (e *databaseExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Database)
	if !ok {
		return managed.ExternalUpdate{}, fmt.Errorf(errNotDatabase)
	}

	fp := cr.Spec.ForProvider
	externalName := meta.GetExternalName(cr)

	db, err := e.client.Databases.Get(ctx, fp.ProjectID, externalName)
	if err != nil {
		return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", errDatabaseObserve, err)
	}

	req := pgbeam.UpdateDatabaseRequest{}
	needsUpdate := false

	if fp.Host != db.Host {
		req.Host = &fp.Host
		needsUpdate = true
	}
	if fp.Port != db.Port {
		req.Port = &fp.Port
		needsUpdate = true
	}
	if fp.Name != db.Name {
		req.Name = &fp.Name
		needsUpdate = true
	}
	if fp.Username != db.Username {
		req.Username = &fp.Username
		needsUpdate = true
	}
	if fp.SSLMode != db.SSLMode {
		req.SSLMode = &fp.SSLMode
		needsUpdate = true
	}
	if fp.Role != db.Role {
		req.Role = &fp.Role
		needsUpdate = true
	}
	if fp.PoolRegion != nil {
		req.PoolRegion = fp.PoolRegion
		needsUpdate = true
	}
	if fp.CacheConfig != nil {
		req.CacheConfig = &pgbeam.CacheConfig{
			Enabled: fp.CacheConfig.Enabled, TTLSeconds: fp.CacheConfig.TTLSeconds,
			MaxEntries: fp.CacheConfig.MaxEntries, SWRSeconds: fp.CacheConfig.SWRSeconds,
		}
		needsUpdate = true
	}
	if fp.PoolConfig != nil {
		req.PoolConfig = &pgbeam.PoolConfig{
			PoolSize: fp.PoolConfig.PoolSize, MinPoolSize: fp.PoolConfig.MinPoolSize,
			PoolMode: fp.PoolConfig.PoolMode,
		}
		needsUpdate = true
	}

	password, err := e.connector.getSecretValue(ctx, fp.PasswordSecretRef)
	if err != nil {
		return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", errDatabasePassword, err)
	}
	req.Password = &password
	needsUpdate = true

	if needsUpdate {
		if _, err := e.client.Databases.Update(ctx, fp.ProjectID, externalName, req); err != nil {
			return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", errDatabaseUpdate, err)
		}
	}
	return managed.ExternalUpdate{}, nil
}

func (e *databaseExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Database)
	if !ok {
		return managed.ExternalDelete{}, fmt.Errorf(errNotDatabase)
	}
	if err := e.client.Databases.Delete(ctx, cr.Spec.ForProvider.ProjectID, meta.GetExternalName(cr)); err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, fmt.Errorf("%s: %w", errDatabaseDelete, err)
	}
	return managed.ExternalDelete{}, nil
}

func databaseObservation(db *pgbeam.Database) v1alpha1.DatabaseAtProvider {
	obs := v1alpha1.DatabaseAtProvider{ID: db.ID, CreatedAt: db.CreatedAt, UpdatedAt: db.UpdatedAt}
	if db.ConnectionString != nil {
		obs.ConnectionString = *db.ConnectionString
	}
	return obs
}

func isDatabaseUpToDate(fp v1alpha1.DatabaseForProvider, db *pgbeam.Database) bool {
	if fp.Host != db.Host || fp.Port != db.Port || fp.Name != db.Name || fp.Username != db.Username {
		return false
	}
	if fp.SSLMode != db.SSLMode || fp.Role != db.Role {
		return false
	}
	if fp.PoolRegion != nil {
		if db.PoolRegion == nil || *fp.PoolRegion != *db.PoolRegion {
			return false
		}
	}
	if fp.CacheConfig != nil {
		if fp.CacheConfig.Enabled != db.CacheConfig.Enabled || fp.CacheConfig.TTLSeconds != db.CacheConfig.TTLSeconds ||
			fp.CacheConfig.MaxEntries != db.CacheConfig.MaxEntries || fp.CacheConfig.SWRSeconds != db.CacheConfig.SWRSeconds {
			return false
		}
	}
	if fp.PoolConfig != nil {
		if fp.PoolConfig.PoolSize != db.PoolConfig.PoolSize || fp.PoolConfig.MinPoolSize != db.PoolConfig.MinPoolSize ||
			fp.PoolConfig.PoolMode != db.PoolConfig.PoolMode {
			return false
		}
	}
	return true
}
