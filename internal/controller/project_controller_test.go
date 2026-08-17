package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pgbeam/provider-pgbeam/apis/v1alpha1"
	pgbeam "go.pgbeam.com/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newProjectExternal(t *testing.T, srv *httptest.Server) *projectExternal {
	t.Helper()
	kube := fake.NewClientBuilder().WithScheme(newTestScheme(t)).
		WithObjects(apiKeySecret("crossplane-system", "db-creds", "password", "s3cret")).
		Build()
	conn := &connector{kube: kube, usage: noopTracker()}
	client := pgbeam.NewClient(&pgbeam.ClientOptions{APIKey: "pgb_test", BaseURL: srv.URL})
	return &projectExternal{client: client, kube: kube, connector: conn}
}

// TestProjectExternal_Observe_NoExternalName asserts an unnamed resource is
// reported as not existing, triggering a Create.
func TestProjectExternal_Observe_NoExternalName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("API should not be called when external name is empty")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	e := newProjectExternal(t, srv)
	obs, err := e.Observe(context.Background(), &v1alpha1.Project{})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false for empty external name")
	}
}

// TestProjectExternal_Observe_WrongType asserts a non-Project managed resource
// is rejected.
func TestProjectExternal_Observe_WrongType(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	e := newProjectExternal(t, srv)
	_, err := e.Observe(context.Background(), &v1alpha1.Database{})
	if err == nil {
		t.Fatal("expected error for non-Project managed resource")
	}
}

// TestProjectExternal_Observe_Existing drives Observe against a mocked GET and
// asserts observation + up-to-date reporting.
func TestProjectExternal_Observe_Existing(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/projects/prj_obs" {
			t.Errorf("path = %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"prj_obs","org_id":"org_1","name":"Watched","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	e := newProjectExternal(t, srv)
	cr := &v1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "watched"},
		Spec:       v1alpha1.ProjectSpec{ForProvider: v1alpha1.ProjectForProvider{Name: "Watched"}},
	}
	meta.SetExternalName(cr, "prj_obs")

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if !obs.ResourceExists {
		t.Error("ResourceExists should be true")
	}
	if !obs.ResourceUpToDate {
		t.Error("ResourceUpToDate should be true (name matches)")
	}
	if cr.Status.AtProvider.ID != "prj_obs" {
		t.Errorf("AtProvider.ID = %q", cr.Status.AtProvider.ID)
	}
	// Observe should set the Available condition.
	if cr.GetCondition(xpv1.TypeReady).Status != "True" {
		t.Errorf("Ready condition = %v, want True", cr.GetCondition(xpv1.TypeReady).Status)
	}
}

// TestProjectExternal_Observe_NotFound asserts a 404 reports the resource as
// not existing (drift → recreate) rather than erroring.
func TestProjectExternal_Observe_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"gone"}}`))
	}))
	defer srv.Close()

	e := newProjectExternal(t, srv)
	cr := &v1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "gone"}}
	meta.SetExternalName(cr, "prj_gone")

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false on 404")
	}
}

// TestProjectExternal_Create drives the create path: it reads the DB password
// secret, POSTs the project, and records the external name + connection details.
func TestProjectExternal_Create(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/projects" {
			t.Errorf("unexpected %s %s", req.Method, req.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"project":{"id":"prj_created","org_id":"org_1","name":"Created","status":"active","proxy_host":"c.proxy.pgbeam.app","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"},"database":{"id":"db_1"}}`))
	}))
	defer srv.Close()

	e := newProjectExternal(t, srv)
	cr := &v1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "created"},
		Spec: v1alpha1.ProjectSpec{ForProvider: v1alpha1.ProjectForProvider{
			OrgID: "org_1",
			Name:  "Created",
			Database: v1alpha1.ProjectDatabaseSpec{
				Host:              "db.internal",
				Port:              5432,
				Name:              "app",
				Username:          "app",
				PasswordSecretRef: secretRef("crossplane-system", "db-creds", "password"),
			},
		}},
	}

	creation, err := e.Create(context.Background(), cr)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := meta.GetExternalName(cr); got != "prj_created" {
		t.Errorf("external name = %q, want prj_created", got)
	}
	if cr.Status.AtProvider.PrimaryDatabaseID != "db_1" {
		t.Errorf("PrimaryDatabaseID = %q, want db_1", cr.Status.AtProvider.PrimaryDatabaseID)
	}
	if got := string(creation.ConnectionDetails["proxyHost"]); got != "c.proxy.pgbeam.app" {
		t.Errorf("proxyHost conn detail = %q", got)
	}
}

// TestProjectExternal_Create_MissingPasswordSecret asserts the create fails
// cleanly when the referenced password secret is absent.
func TestProjectExternal_Create_MissingPasswordSecret(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("API should not be called when the password secret is missing")
	}))
	defer srv.Close()

	e := newProjectExternal(t, srv)
	cr := &v1alpha1.Project{
		Spec: v1alpha1.ProjectSpec{ForProvider: v1alpha1.ProjectForProvider{
			OrgID: "org_1",
			Name:  "Created",
			Database: v1alpha1.ProjectDatabaseSpec{
				Host:              "db.internal",
				Port:              5432,
				Name:              "app",
				Username:          "app",
				PasswordSecretRef: secretRef("crossplane-system", "absent", "password"),
			},
		}},
	}
	if _, err := e.Create(context.Background(), cr); err == nil {
		t.Fatal("expected error for missing password secret")
	}
}

// TestProjectExternal_Delete_NotFound asserts deleting an already-gone project
// is a no-op success.
func TestProjectExternal_Delete_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", req.Method)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	e := newProjectExternal(t, srv)
	cr := &v1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "gone"}}
	meta.SetExternalName(cr, "prj_gone")

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete should ignore 404: %v", err)
	}
}

// TestProjectExternal_Delete drives a successful delete.
func TestProjectExternal_Delete(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/projects/prj_del" {
			t.Errorf("path = %s", req.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	e := newProjectExternal(t, srv)
	cr := &v1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "del"}}
	meta.SetExternalName(cr, "prj_del")

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

var _ resource.Managed = (*v1alpha1.Project)(nil)
