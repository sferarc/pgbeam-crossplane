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
	errNotProject      = "managed resource is not a Project"
	errProjectCreate   = "cannot create Project"
	errProjectUpdate   = "cannot update Project"
	errProjectDelete   = "cannot delete Project"
	errProjectObserve  = "cannot observe Project"
	errProjectPassword = "cannot read database password"
)

func SetupProject(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ProjectGroupKind)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Project{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.ProjectGroupVersionKind),
			managed.WithExternalConnecter(&projectConnecter{
				connector: connector{
					kube:  mgr.GetClient(),
					usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &v1alpha1.ProviderConfigUsage{}),
				},
			}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		))
}

type projectConnecter struct{ connector }

func (c *projectConnecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cl, err := c.getClient(ctx, mg)
	if err != nil {
		return nil, err
	}
	return &projectExternal{client: cl, kube: c.kube, connector: &c.connector}, nil
}

type projectExternal struct {
	client    *pgbeam.Client
	kube      client.Client
	connector *connector
}

func (e *projectExternal) Disconnect(_ context.Context) error { return nil }

func (e *projectExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Project)
	if !ok {
		return managed.ExternalObservation{}, fmt.Errorf(errNotProject)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	project, err := e.client.Projects.Get(ctx, externalName)
	if err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", errProjectObserve, err)
	}

	cr.Status.AtProvider = projectObservation(project)
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isProjectUpToDate(cr.Spec.ForProvider, project),
	}, nil
}

func (e *projectExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Project)
	if !ok {
		return managed.ExternalCreation{}, fmt.Errorf(errNotProject)
	}

	fp := cr.Spec.ForProvider
	password, err := e.connector.getSecretValue(ctx, fp.Database.PasswordSecretRef)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", errProjectPassword, err)
	}

	req := pgbeam.CreateProjectRequest{
		Name:  fp.Name,
		OrgID: fp.OrgID,
		Cloud: fp.Cloud,
		Database: pgbeam.CreateDatabaseRequest{
			Host: fp.Database.Host, Port: fp.Database.Port,
			Name: fp.Database.Name, Username: fp.Database.Username,
			Password: password, SSLMode: fp.Database.SSLMode, Role: fp.Database.Role,
		},
	}
	if fp.Description != nil {
		req.Description = fp.Description
	}
	if fp.Tags != nil {
		req.Tags = fp.Tags
	}
	if fp.Database.PoolRegion != nil {
		req.Database.PoolRegion = fp.Database.PoolRegion
	}
	if fp.Database.CacheConfig != nil {
		req.Database.CacheConfig = &pgbeam.CacheConfig{
			Enabled: fp.Database.CacheConfig.Enabled, TTLSeconds: fp.Database.CacheConfig.TTLSeconds,
			MaxEntries: fp.Database.CacheConfig.MaxEntries, SWRSeconds: fp.Database.CacheConfig.SWRSeconds,
		}
	}
	if fp.Database.PoolConfig != nil {
		req.Database.PoolConfig = &pgbeam.PoolConfig{
			PoolSize: fp.Database.PoolConfig.PoolSize, MinPoolSize: fp.Database.PoolConfig.MinPoolSize,
			PoolMode: fp.Database.PoolConfig.PoolMode,
		}
	}

	resp, err := e.client.Projects.Create(ctx, req)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", errProjectCreate, err)
	}

	// Rate-limit fields are not supported in CreateProjectRequest.
	// If specified, issue an immediate update after creation.
	updateReq := pgbeam.UpdateProjectRequest{}
	needsPostCreateUpdate := false
	if fp.QueriesPerSecond != nil {
		updateReq.QueriesPerSecond = fp.QueriesPerSecond
		needsPostCreateUpdate = true
	}
	if fp.BurstSize != nil {
		updateReq.BurstSize = fp.BurstSize
		needsPostCreateUpdate = true
	}
	if fp.MaxConnections != nil {
		updateReq.MaxConnections = fp.MaxConnections
		needsPostCreateUpdate = true
	}
	if needsPostCreateUpdate {
		updated, err := e.client.Projects.Update(ctx, resp.Project.ID, updateReq)
		if err != nil {
			return managed.ExternalCreation{}, fmt.Errorf("%s: %w", errProjectUpdate, err)
		}
		resp.Project = *updated
	}

	meta.SetExternalName(cr, resp.Project.ID)
	cr.Status.AtProvider = projectObservation(&resp.Project)
	if resp.Database != nil {
		cr.Status.AtProvider.PrimaryDatabaseID = resp.Database.ID
	}

	connDetails := managed.ConnectionDetails{}
	if resp.Project.ProxyHost != "" {
		connDetails["proxyHost"] = []byte(resp.Project.ProxyHost)
	}
	return managed.ExternalCreation{ConnectionDetails: connDetails}, nil
}

func (e *projectExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Project)
	if !ok {
		return managed.ExternalUpdate{}, fmt.Errorf(errNotProject)
	}

	fp := cr.Spec.ForProvider
	externalName := meta.GetExternalName(cr)

	project, err := e.client.Projects.Get(ctx, externalName)
	if err != nil {
		return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", errProjectObserve, err)
	}

	req := pgbeam.UpdateProjectRequest{}
	needsUpdate := false

	if fp.Name != project.Name {
		req.Name = &fp.Name
		needsUpdate = true
	}
	if fp.Description != nil && (project.Description == nil || *fp.Description != *project.Description) {
		req.Description = fp.Description
		needsUpdate = true
	}
	if fp.Tags != nil {
		req.Tags = &fp.Tags
		needsUpdate = true
	}
	if fp.QueriesPerSecond != nil && *fp.QueriesPerSecond != project.QueriesPerSecond {
		req.QueriesPerSecond = fp.QueriesPerSecond
		needsUpdate = true
	}
	if fp.BurstSize != nil && *fp.BurstSize != project.BurstSize {
		req.BurstSize = fp.BurstSize
		needsUpdate = true
	}
	if fp.MaxConnections != nil && *fp.MaxConnections != project.MaxConnections {
		req.MaxConnections = fp.MaxConnections
		needsUpdate = true
	}

	if needsUpdate {
		if _, err := e.client.Projects.Update(ctx, externalName, req); err != nil {
			return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", errProjectUpdate, err)
		}
	}
	return managed.ExternalUpdate{}, nil
}

func (e *projectExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Project)
	if !ok {
		return managed.ExternalDelete{}, fmt.Errorf(errNotProject)
	}
	if err := e.client.Projects.Delete(ctx, meta.GetExternalName(cr)); err != nil {
		if pgbeam.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, fmt.Errorf("%s: %w", errProjectDelete, err)
	}
	return managed.ExternalDelete{}, nil
}

func projectObservation(p *pgbeam.Project) v1alpha1.ProjectAtProvider {
	return v1alpha1.ProjectAtProvider{
		ID: p.ID, ProxyHost: p.ProxyHost, Status: p.Status,
		DatabaseCount: p.DatabaseCount, ActiveConnections: p.ActiveConnections,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func isProjectUpToDate(fp v1alpha1.ProjectForProvider, p *pgbeam.Project) bool {
	if fp.Name != p.Name {
		return false
	}
	if fp.Description != nil && (p.Description == nil || *fp.Description != *p.Description) {
		return false
	}
	if fp.Tags != nil {
		if len(fp.Tags) != len(p.Tags) {
			return false
		}
		for i, t := range fp.Tags {
			if t != p.Tags[i] {
				return false
			}
		}
	}
	if fp.QueriesPerSecond != nil && *fp.QueriesPerSecond != p.QueriesPerSecond {
		return false
	}
	if fp.BurstSize != nil && *fp.BurstSize != p.BurstSize {
		return false
	}
	if fp.MaxConnections != nil && *fp.MaxConnections != p.MaxConnections {
		return false
	}
	return true
}
