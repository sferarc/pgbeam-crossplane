package controller

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/sferarc/pgbeam-crossplane/apis/v1alpha1"
	pgbeam "go.pgbeam.com/sdk"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func desiredDatabase() v1alpha1.DatabaseForProvider {
	return v1alpha1.DatabaseForProvider{
		ProjectID:       "prj_1",
		Host:            "db.internal",
		Port:            5432,
		Name:            "app",
		Username:        "app_rw",
		SSLMode:         "verify-full",
		Role:            "replica",
		PoolRegion:      ptr("eu-west-1"),
		QueryTimeoutMs:  ptr(30000),
		AutoReadRouting: ptr(true),
		CacheConfig:     &v1alpha1.CacheConfigSpec{Enabled: true, TTLSeconds: 120, MaxEntries: 5000, SWRSeconds: 15},
		PoolConfig:      &v1alpha1.PoolConfigSpec{PoolSize: 20, MinPoolSize: 2, PoolMode: "transaction", MaxActive: ptr(100)},
		PasswordSecretRef: secretRef(
			"crossplane-system", "db-creds", "password",
		),
	}
}

func databaseMatching() *pgbeam.Database {
	return &pgbeam.Database{
		Id:              "db_1",
		ProjectId:       "prj_1",
		Host:            "db.internal",
		Port:            5432,
		Name:            "app",
		Username:        "app_rw",
		SslMode:         "verify-full",
		Role:            ptr(pgbeam.DatabaseRole("replica")),
		PoolRegion:      ptr("eu-west-1"),
		QueryTimeoutMs:  ptr(30000),
		AutoReadRouting: ptr(true),
		CacheConfig:     pgbeam.CacheConfig{Enabled: true, TtlSeconds: 120, MaxEntries: 5000, SwrSeconds: 15},
		PoolConfig:      pgbeam.PoolConfig{PoolSize: 20, MinPoolSize: 2, PoolMode: "transaction", MaxActive: ptr(100)},
	}
}

func TestDatabaseIdenticalIsUpToDate(t *testing.T) {
	t.Parallel()
	if !isDatabaseUpToDate(desiredDatabase(), databaseMatching()) {
		t.Fatal("identical spec and API object reported as drift")
	}
}

func TestDatabaseFieldDrift(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*pgbeam.Database){
		"Host":            func(d *pgbeam.Database) { d.Host = "db.attacker.example" },
		"Port":            func(d *pgbeam.Database) { d.Port = 6543 },
		"Name":            func(d *pgbeam.Database) { d.Name = "other" },
		"Username":        func(d *pgbeam.Database) { d.Username = "postgres" },
		"SSLMode":         func(d *pgbeam.Database) { d.SslMode = "disable" },
		"Role":            func(d *pgbeam.Database) { d.Role = nil },
		"PoolRegion":      func(d *pgbeam.Database) { d.PoolRegion = ptr("us-east-1") },
		"QueryTimeoutMs":  func(d *pgbeam.Database) { d.QueryTimeoutMs = ptr(0) },
		"AutoReadRouting": func(d *pgbeam.Database) { d.AutoReadRouting = ptr(false) },
		"CacheConfig":     func(d *pgbeam.Database) { d.CacheConfig.Enabled = false },
		"PoolConfig":      func(d *pgbeam.Database) { d.PoolConfig.PoolSize = 1 },
	}
	// The password is write-only, so Update always resends it.
	requireCases(t, reflect.TypeFor[v1alpha1.DatabaseForProvider](), cases, "ProjectID", "PasswordSecretRef")

	cache := map[string]func(*pgbeam.CacheConfig){
		"Enabled":    func(c *pgbeam.CacheConfig) { c.Enabled = false },
		"TTLSeconds": func(c *pgbeam.CacheConfig) { c.TtlSeconds = 60 },
		"MaxEntries": func(c *pgbeam.CacheConfig) { c.MaxEntries = 10000 },
		"SWRSeconds": func(c *pgbeam.CacheConfig) { c.SwrSeconds = 30 },
	}
	requireCases(t, reflect.TypeFor[v1alpha1.CacheConfigSpec](), cache)
	for name, edit := range cache {
		cases["CacheConfig."+name] = func(d *pgbeam.Database) { edit(&d.CacheConfig) }
	}

	pool := map[string]func(*pgbeam.PoolConfig){
		"PoolSize":    func(p *pgbeam.PoolConfig) { p.PoolSize = 10 },
		"MinPoolSize": func(p *pgbeam.PoolConfig) { p.MinPoolSize = 1 },
		"PoolMode":    func(p *pgbeam.PoolConfig) { p.PoolMode = "session" },
		"MaxActive":   func(p *pgbeam.PoolConfig) { p.MaxActive = ptr(200) },
	}
	requireCases(t, reflect.TypeFor[v1alpha1.PoolConfigSpec](), pool)
	for name, edit := range pool {
		cases["PoolConfig."+name] = func(d *pgbeam.Database) { edit(&d.PoolConfig) }
	}

	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := databaseMatching()
			edit(d)
			if isDatabaseUpToDate(desiredDatabase(), d) {
				t.Errorf("edit to %s not detected", name)
			}
		})
	}
}

// Database updates are a PATCH, which cannot reset a field the body leaves
// out, so an omitted optional field is unmanaged rather than held at a
// default. Comparing it would report drift that Update can never clear.
func TestDatabaseOmittedFieldsAreUnmanaged(t *testing.T) {
	t.Parallel()

	minimal := v1alpha1.DatabaseForProvider{
		ProjectID:         "prj_1",
		Host:              "db.internal",
		Port:              5432,
		Name:              "app",
		Username:          "app_rw",
		PasswordSecretRef: secretRef("crossplane-system", "db-creds", "password"),
	}

	// databaseMatching holds every one of these away from its default.
	want := []string{"SSLMode", "Role", "PoolRegion", "QueryTimeoutMs", "AutoReadRouting", "CacheConfig", "PoolConfig"}
	if got := optionalFields(reflect.TypeFor[v1alpha1.DatabaseForProvider]()); !slices.Equal(got, want) {
		t.Fatalf("optional fields = %v, want %v", got, want)
	}

	if !isDatabaseUpToDate(minimal, databaseMatching()) {
		t.Error("omitted optional fields reported as drift on a PATCH resource")
	}
}

// The CRD defaults maxActive inside a declared poolConfig, but a manifest
// applied without defaulting can still leave it nil.
func TestDatabasePoolConfigWithoutMaxActive(t *testing.T) {
	t.Parallel()

	fp := desiredDatabase()
	fp.PoolConfig.MaxActive = nil
	if !isDatabaseUpToDate(fp, databaseMatching()) {
		t.Error("nil maxActive reported as drift")
	}
}

func newDatabaseExternal(t *testing.T, api *fakeAPI) *databaseExternal {
	t.Helper()
	kube := fake.NewClientBuilder().WithScheme(newTestScheme(t)).
		WithObjects(apiKeySecret("crossplane-system", "db-creds", "password", "s3cret")).
		Build()
	return &databaseExternal{client: api.client(), kube: kube, connector: &connector{kube: kube, usage: noopTracker()}}
}

func TestDatabaseUpdateSendsEveryDriftedField(t *testing.T) {
	t.Parallel()

	stale := &pgbeam.Database{
		Id:              "db_1",
		ProjectId:       "prj_1",
		Host:            "old.internal",
		Port:            6543,
		Name:            "old",
		Username:        "old",
		SslMode:         "disable",
		Role:            ptr(pgbeam.DatabaseRole("primary")),
		PoolRegion:      ptr("us-east-1"),
		QueryTimeoutMs:  ptr(0),
		AutoReadRouting: ptr(false),
		CacheConfig:     pgbeam.CacheConfig{Enabled: false, TtlSeconds: 60, MaxEntries: 10000, SwrSeconds: 30},
		PoolConfig:      pgbeam.PoolConfig{PoolSize: 10, MinPoolSize: 1, PoolMode: "session", MaxActive: ptr(200)},
	}
	api := newFakeAPI(t, stale)
	e := newDatabaseExternal(t, api)
	cr := &v1alpha1.Database{Spec: v1alpha1.DatabaseSpec{ForProvider: desiredDatabase()}}
	meta.SetExternalName(cr, "db_1")

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update: %v", err)
	}

	call, keys := api.onlyWrite(t, "PATCH")
	for _, k := range jsonFieldNames(reflect.TypeFor[pgbeam.UpdateDatabaseRequest]()) {
		if _, ok := keys[k]; !ok {
			t.Errorf("PATCH body has no %q although every field drifted", k)
		}
	}
	// The PATCH merged over the stored row must leave it up to date.
	applied := *stale
	if err := json.Unmarshal(call.body, &applied); err != nil {
		t.Fatal(err)
	}
	if !isDatabaseUpToDate(desiredDatabase(), &applied) {
		t.Errorf("PATCH body %s does not carry the desired state", call.body)
	}
}

// An omitted sslMode used to be sent as "" whenever any other field drifted,
// which the API rejects, so the whole update failed.
func TestDatabaseUpdateLeavesOmittedSSLModeAlone(t *testing.T) {
	t.Parallel()

	stale := databaseMatching()
	stale.SslMode = "require"
	stale.Host = "old.internal"
	api := newFakeAPI(t, stale)
	e := newDatabaseExternal(t, api)
	fp := desiredDatabase()
	fp.SSLMode = ""
	cr := &v1alpha1.Database{Spec: v1alpha1.DatabaseSpec{ForProvider: fp}}
	meta.SetExternalName(cr, "db_1")

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update: %v", err)
	}

	_, keys := api.onlyWrite(t, "PATCH")
	if v, ok := keys["ssl_mode"]; ok {
		t.Errorf("PATCH body carries ssl_mode %s for an omitted field", v)
	}
	if string(keys["host"]) != `"db.internal"` {
		t.Errorf("host = %s, want the drifted value corrected", keys["host"])
	}
	if string(keys["password"]) != `"s3cret"` {
		t.Errorf("password = %s, want the referenced secret value", keys["password"])
	}
}
