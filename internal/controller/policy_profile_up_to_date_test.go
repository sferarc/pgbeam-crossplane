package controller

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/sferarc/pgbeam-crossplane/apis/v1alpha1"
	pgbeam "go.pgbeam.com/sdk"
)

func desiredPolicyProfile() v1alpha1.PolicyProfileForProvider {
	return v1alpha1.PolicyProfileForProvider{
		ProjectID:  "prj_1",
		Name:       "agents",
		AccessMode: "read_write",
		StatementRules: &v1alpha1.StatementRulesSpec{
			Allow: &[]string{"select", "insert"},
			Deny:  &[]string{"ddl"},
		},
		TableAllowlist: []string{"orders", "billing.invoices"},
		TableDenylist:  []string{"secrets"},
		MaskingRules: []v1alpha1.MaskingRuleParameters{
			{Table: "users", Column: "email", Kind: "redact"},
			{Table: "users", Column: "ssn", Kind: "hash"},
		},
		BudgetQueriesPerHour: ptr(100),
		BudgetQueriesPerDay:  ptr(1000),
		MaxRows:              ptr(500),
		StatementTimeoutMs:   ptr(5000),
		RowFilters: []v1alpha1.RowFilterParameters{
			{Table: "orders", Predicate: "tenant_id = 1"},
			{Table: "billing.invoices", Predicate: "tenant_id = 1"},
		},
		WriteMode:              "rollback",
		ApprovalMode:           "writes",
		ApprovalAutoMaxRows:    ptr(10),
		ApprovalTimeoutSeconds: ptr(600),
		MigrationSafety:        "block",
		EgressBytesPerDay:      ptr(int64(1 << 20)),
		MaxAffectedRows:        ptr(50),
		ContentScanMode:        "block",
		ContentScanMaxBytes:    ptr(int64(1 << 16)),
		QueryMode:              "templates",
	}
}

// policyProfileMatching is what the API returns for desiredPolicyProfile.
func policyProfileMatching() *pgbeam.PolicyProfile {
	return &pgbeam.PolicyProfile{
		Id:                     "pol_1",
		ProjectId:              "prj_1",
		Name:                   "agents",
		AccessMode:             pgbeam.PolicyProfileAccessModeReadWrite,
		StatementRules:         &pgbeam.StatementRules{Allow: &[]string{"select", "insert"}, Deny: &[]string{"ddl"}},
		TableAllowlist:         []string{"orders", "billing.invoices"},
		TableDenylist:          []string{"secrets"},
		MaskingRules:           []pgbeam.MaskingRule{{Table: "users", Column: "email", Kind: "redact"}, {Table: "users", Column: "ssn", Kind: "hash"}},
		BudgetQueriesPerHour:   ptr(100),
		BudgetQueriesPerDay:    ptr(1000),
		MaxRows:                ptr(500),
		StatementTimeoutMs:     ptr(5000),
		RowFilters:             &[]pgbeam.RowFilter{{Table: "orders", Predicate: "tenant_id = 1"}, {Table: "billing.invoices", Predicate: "tenant_id = 1"}},
		WriteMode:              ptr(pgbeam.PolicyProfileWriteMode("rollback")),
		ApprovalMode:           ptr(pgbeam.PolicyProfileApprovalMode("writes")),
		ApprovalAutoMaxRows:    ptr(10),
		ApprovalTimeoutSeconds: ptr(600),
		MigrationSafety:        ptr(pgbeam.PolicyProfileMigrationSafety("block")),
		EgressBytesPerDay:      ptr(int64(1 << 20)),
		MaxAffectedRows:        ptr(50),
		ContentScanMode:        ptr(pgbeam.PolicyProfileContentScanMode("block")),
		ContentScanMaxBytes:    ptr(int64(1 << 16)),
		QueryMode:              pgbeam.PolicyProfileQueryModeTemplates,
	}
}

// policyProfileDefaults is what the API returns for a profile created with
// only a name: every optional column at its stored default.
func policyProfileDefaults() *pgbeam.PolicyProfile {
	return &pgbeam.PolicyProfile{
		Id:                     "pol_1",
		ProjectId:              "prj_1",
		Name:                   "agents",
		AccessMode:             pgbeam.PolicyProfileAccessModeReadOnly,
		TableAllowlist:         []string{},
		TableDenylist:          []string{},
		BudgetQueriesPerHour:   ptr(0),
		BudgetQueriesPerDay:    ptr(0),
		MaxRows:                ptr(0),
		StatementTimeoutMs:     ptr(0),
		RowFilters:             &[]pgbeam.RowFilter{},
		WriteMode:              ptr(pgbeam.PolicyProfileWriteMode("normal")),
		ApprovalMode:           ptr(pgbeam.PolicyProfileApprovalMode("off")),
		ApprovalAutoMaxRows:    ptr(0),
		ApprovalTimeoutSeconds: ptr(300),
		MigrationSafety:        ptr(pgbeam.PolicyProfileMigrationSafety("off")),
		EgressBytesPerDay:      ptr(int64(0)),
		MaxAffectedRows:        ptr(0),
		ContentScanMode:        ptr(pgbeam.PolicyProfileContentScanMode("off")),
		ContentScanMaxBytes:    ptr(int64(0)),
		QueryMode:              pgbeam.PolicyProfileQueryModeSql,
	}
}

func TestPolicyProfileIdenticalIsUpToDate(t *testing.T) {
	t.Parallel()
	if !isPolicyProfileUpToDate(desiredPolicyProfile(), policyProfileMatching()) {
		t.Fatal("identical spec and API object reported as drift")
	}
}

// Each case is a console edit to one field; every one must be reverted.
func TestPolicyProfileFieldDrift(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*pgbeam.PolicyProfile){
		"Name":                 func(p *pgbeam.PolicyProfile) { p.Name = "renamed" },
		"AccessMode":           func(p *pgbeam.PolicyProfile) { p.AccessMode = pgbeam.PolicyProfileAccessModeReadOnly },
		"StatementRules":       func(p *pgbeam.PolicyProfile) { p.StatementRules.Deny = nil },
		"TableAllowlist":       func(p *pgbeam.PolicyProfile) { p.TableAllowlist = append(p.TableAllowlist, "users") },
		"TableDenylist":        func(p *pgbeam.PolicyProfile) { p.TableDenylist = nil },
		"MaskingRules":         func(p *pgbeam.PolicyProfile) { p.MaskingRules[1].Kind = "null" },
		"BudgetQueriesPerHour": func(p *pgbeam.PolicyProfile) { p.BudgetQueriesPerHour = ptr(101) },
		"BudgetQueriesPerDay":  func(p *pgbeam.PolicyProfile) { p.BudgetQueriesPerDay = ptr(0) },
		"MaxRows":              func(p *pgbeam.PolicyProfile) { p.MaxRows = nil },
		"StatementTimeoutMs":   func(p *pgbeam.PolicyProfile) { p.StatementTimeoutMs = ptr(60000) },
		"RowFilters":           func(p *pgbeam.PolicyProfile) { (*p.RowFilters)[0].Predicate = "true" },
		"WriteMode":            func(p *pgbeam.PolicyProfile) { p.WriteMode = ptr(pgbeam.PolicyProfileWriteMode("normal")) },
		"ApprovalMode":         func(p *pgbeam.PolicyProfile) { p.ApprovalMode = ptr(pgbeam.PolicyProfileApprovalMode("off")) },
		"ApprovalAutoMaxRows":  func(p *pgbeam.PolicyProfile) { p.ApprovalAutoMaxRows = ptr(100000) },
		"ApprovalTimeoutSeconds": func(p *pgbeam.PolicyProfile) {
			p.ApprovalTimeoutSeconds = ptr(86400)
		},
		"MigrationSafety":   func(p *pgbeam.PolicyProfile) { p.MigrationSafety = ptr(pgbeam.PolicyProfileMigrationSafety("warn")) },
		"EgressBytesPerDay": func(p *pgbeam.PolicyProfile) { p.EgressBytesPerDay = ptr(int64(0)) },
		"MaxAffectedRows":   func(p *pgbeam.PolicyProfile) { p.MaxAffectedRows = ptr(51) },
		"ContentScanMode": func(p *pgbeam.PolicyProfile) {
			p.ContentScanMode = ptr(pgbeam.PolicyProfileContentScanMode("annotate"))
		},
		"ContentScanMaxBytes": func(p *pgbeam.PolicyProfile) { p.ContentScanMaxBytes = nil },
		"QueryMode":           func(p *pgbeam.PolicyProfile) { p.QueryMode = pgbeam.PolicyProfileQueryModeSql },
	}
	requireCases(t, reflect.TypeFor[v1alpha1.PolicyProfileForProvider](), cases, "ProjectID")

	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := policyProfileMatching()
			edit(p)
			if isPolicyProfileUpToDate(desiredPolicyProfile(), p) {
				t.Errorf("edit to %s not detected", name)
			}
		})
	}
}

// A PUT resets every omitted field, so an omitted field is managed at the API
// default: up to date at the default, drift anywhere else.
func TestPolicyProfileOmittedFieldsCompareAgainstDefault(t *testing.T) {
	t.Parallel()

	minimal := v1alpha1.PolicyProfileForProvider{ProjectID: "prj_1", Name: "agents"}
	if !isPolicyProfileUpToDate(minimal, policyProfileDefaults()) {
		t.Fatal("minimal spec against API defaults reported as drift")
	}
	// An API that omits a column it holds at the default is still at the default.
	sparse := &pgbeam.PolicyProfile{Name: "agents", AccessMode: pgbeam.PolicyProfileAccessModeReadOnly, QueryMode: pgbeam.PolicyProfileQueryModeSql}
	if !isPolicyProfileUpToDate(minimal, sparse) {
		t.Fatal("minimal spec against a sparse response reported as drift")
	}

	offDefault := map[string]func(*pgbeam.PolicyProfile){
		"AccessMode":     func(p *pgbeam.PolicyProfile) { p.AccessMode = pgbeam.PolicyProfileAccessModeReadWrite },
		"StatementRules": func(p *pgbeam.PolicyProfile) { p.StatementRules = &pgbeam.StatementRules{Deny: &[]string{"ddl"}} },
		"TableAllowlist": func(p *pgbeam.PolicyProfile) { p.TableAllowlist = []string{"orders"} },
		"TableDenylist":  func(p *pgbeam.PolicyProfile) { p.TableDenylist = []string{"secrets"} },
		"MaskingRules": func(p *pgbeam.PolicyProfile) {
			p.MaskingRules = []pgbeam.MaskingRule{{Table: "t", Column: "c", Kind: "redact"}}
		},
		"BudgetQueriesPerHour":   func(p *pgbeam.PolicyProfile) { p.BudgetQueriesPerHour = ptr(1) },
		"BudgetQueriesPerDay":    func(p *pgbeam.PolicyProfile) { p.BudgetQueriesPerDay = ptr(1) },
		"MaxRows":                func(p *pgbeam.PolicyProfile) { p.MaxRows = ptr(1) },
		"StatementTimeoutMs":     func(p *pgbeam.PolicyProfile) { p.StatementTimeoutMs = ptr(1) },
		"RowFilters":             func(p *pgbeam.PolicyProfile) { p.RowFilters = &[]pgbeam.RowFilter{{Table: "t", Predicate: "false"}} },
		"WriteMode":              func(p *pgbeam.PolicyProfile) { p.WriteMode = ptr(pgbeam.PolicyProfileWriteMode("sandbox")) },
		"ApprovalMode":           func(p *pgbeam.PolicyProfile) { p.ApprovalMode = ptr(pgbeam.PolicyProfileApprovalMode("all")) },
		"ApprovalAutoMaxRows":    func(p *pgbeam.PolicyProfile) { p.ApprovalAutoMaxRows = ptr(1) },
		"ApprovalTimeoutSeconds": func(p *pgbeam.PolicyProfile) { p.ApprovalTimeoutSeconds = ptr(0) },
		"MigrationSafety":        func(p *pgbeam.PolicyProfile) { p.MigrationSafety = ptr(pgbeam.PolicyProfileMigrationSafety("warn")) },
		"EgressBytesPerDay":      func(p *pgbeam.PolicyProfile) { p.EgressBytesPerDay = ptr(int64(1)) },
		"MaxAffectedRows":        func(p *pgbeam.PolicyProfile) { p.MaxAffectedRows = ptr(1) },
		"ContentScanMode":        func(p *pgbeam.PolicyProfile) { p.ContentScanMode = ptr(pgbeam.PolicyProfileContentScanMode("block")) },
		"ContentScanMaxBytes":    func(p *pgbeam.PolicyProfile) { p.ContentScanMaxBytes = ptr(int64(1)) },
		"QueryMode":              func(p *pgbeam.PolicyProfile) { p.QueryMode = pgbeam.PolicyProfileQueryModeTemplates },
	}
	typ := reflect.TypeFor[v1alpha1.PolicyProfileForProvider]()
	requireCases(t, typ, offDefault, "ProjectID", "Name")
	if got := len(optionalFields(typ)); got != len(offDefault) {
		t.Errorf("%d optional fields, %d cases", got, len(offDefault))
	}

	for name, edit := range offDefault {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := policyProfileDefaults()
			edit(p)
			if isPolicyProfileUpToDate(minimal, p) {
				t.Errorf("omitted %s with a non-default server value not detected", name)
			}
		})
	}
}

// Order is meaningful to the API (it is stored and returned as given), so a
// reorder is drift as much as an add or a remove.
func TestPolicyProfileListDrift(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		edit func(*pgbeam.PolicyProfile)
	}{
		{"tableAllowlist add", func(p *pgbeam.PolicyProfile) { p.TableAllowlist = append(p.TableAllowlist, "users") }},
		{"tableAllowlist remove", func(p *pgbeam.PolicyProfile) { p.TableAllowlist = p.TableAllowlist[:1] }},
		{"tableAllowlist reorder", func(p *pgbeam.PolicyProfile) { p.TableAllowlist = []string{"billing.invoices", "orders"} }},
		{"tableDenylist add", func(p *pgbeam.PolicyProfile) { p.TableDenylist = append(p.TableDenylist, "audit") }},
		{"tableDenylist remove", func(p *pgbeam.PolicyProfile) { p.TableDenylist = []string{} }},
		{"maskingRules add", func(p *pgbeam.PolicyProfile) {
			p.MaskingRules = append(p.MaskingRules, pgbeam.MaskingRule{Table: "users", Column: "phone", Kind: "null"})
		}},
		{"maskingRules remove", func(p *pgbeam.PolicyProfile) { p.MaskingRules = p.MaskingRules[:1] }},
		{"maskingRules reorder", func(p *pgbeam.PolicyProfile) {
			p.MaskingRules[0], p.MaskingRules[1] = p.MaskingRules[1], p.MaskingRules[0]
		}},
		{"maskingRules table changed", func(p *pgbeam.PolicyProfile) { p.MaskingRules[0].Table = "customers" }},
		{"maskingRules column changed", func(p *pgbeam.PolicyProfile) { p.MaskingRules[0].Column = "name" }},
		{"rowFilters add", func(p *pgbeam.PolicyProfile) {
			*p.RowFilters = append(*p.RowFilters, pgbeam.RowFilter{Table: "users", Predicate: "true"})
		}},
		{"rowFilters remove", func(p *pgbeam.PolicyProfile) { *p.RowFilters = (*p.RowFilters)[:1] }},
		{"rowFilters reorder", func(p *pgbeam.PolicyProfile) {
			rf := *p.RowFilters
			rf[0], rf[1] = rf[1], rf[0]
		}},
		{"rowFilters dropped by the API", func(p *pgbeam.PolicyProfile) { p.RowFilters = nil }},
		{"rowFilters table changed", func(p *pgbeam.PolicyProfile) { (*p.RowFilters)[1].Table = "invoices" }},
		{"statementRules allow add", func(p *pgbeam.PolicyProfile) { *p.StatementRules.Allow = append(*p.StatementRules.Allow, "ddl") }},
		{"statementRules allow reorder", func(p *pgbeam.PolicyProfile) { p.StatementRules.Allow = &[]string{"insert", "select"} }},
		{"statementRules dropped by the API", func(p *pgbeam.PolicyProfile) { p.StatementRules = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := policyProfileMatching()
			tc.edit(p)
			if isPolicyProfileUpToDate(desiredPolicyProfile(), p) {
				t.Errorf("%s not detected", tc.name)
			}
		})
	}
}

// The API stores an empty statementRules object as "{}" and omits it from
// responses, which used to dereference a nil pointer in the comparison.
func TestPolicyProfileEmptyStatementRulesOmittedByAPI(t *testing.T) {
	t.Parallel()

	fp := v1alpha1.PolicyProfileForProvider{ProjectID: "prj_1", Name: "agents", StatementRules: &v1alpha1.StatementRulesSpec{}}
	if !isPolicyProfileUpToDate(fp, policyProfileDefaults()) {
		t.Error("empty statementRules against an omitted object reported as drift")
	}
	fp.StatementRules.Allow = &[]string{"select"}
	if isPolicyProfileUpToDate(fp, policyProfileDefaults()) {
		t.Error("allow list against an omitted object not detected")
	}
}

func TestPolicyProfileUpdateSendsFullDesiredState(t *testing.T) {
	t.Parallel()

	api := newFakeAPI(t, policyProfileDefaults())
	e := &policyProfileExternal{client: api.client()}
	cr := &v1alpha1.PolicyProfile{Spec: v1alpha1.PolicyProfileSpec{ForProvider: desiredPolicyProfile()}}
	meta.SetExternalName(cr, "pol_1")

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update: %v", err)
	}

	call, keys := api.onlyWrite(t, "PUT")
	for _, k := range jsonFieldNames(reflect.TypeFor[pgbeam.PolicyProfileInput]()) {
		if _, ok := keys[k]; !ok {
			t.Errorf("PUT body has no %q; the API would reset it to its default", k)
		}
	}
	// The body, stored as sent, must leave the resource up to date.
	var applied pgbeam.PolicyProfile
	if err := json.Unmarshal(call.body, &applied); err != nil {
		t.Fatal(err)
	}
	if !isPolicyProfileUpToDate(desiredPolicyProfile(), &applied) {
		t.Errorf("PUT body %s does not carry the desired state", call.body)
	}
}

func TestPolicyProfileUpdateSkipsWhenUpToDate(t *testing.T) {
	t.Parallel()

	api := newFakeAPI(t, policyProfileMatching())
	e := &policyProfileExternal{client: api.client()}
	cr := &v1alpha1.PolicyProfile{Spec: v1alpha1.PolicyProfileSpec{ForProvider: desiredPolicyProfile()}}
	meta.SetExternalName(cr, "pol_1")

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if calls := api.writes(); len(calls) != 0 {
		t.Errorf("got %d writes for an up-to-date profile", len(calls))
	}
}
