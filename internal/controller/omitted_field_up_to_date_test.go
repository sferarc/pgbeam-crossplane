package controller

import (
	"testing"

	"github.com/sferarc/pgbeam-crossplane/apis/v1alpha1"
	pgbeam "go.pgbeam.com/sdk"
)

func ptr[T any](v T) *T { return &v }

// A PUT stores the default (or null) for every field it omits, so a field
// removed from the manifest while the server still holds a value is drift.
func TestOmittedFieldIsDriftOnFullReplacement(t *testing.T) {
	t.Parallel()

	cred := "agt_1"
	rule := func() *pgbeam.AnomalyRule {
		return &pgbeam.AnomalyRule{Metric: pgbeam.AnomalyMetric("queries_per_hour"), Enabled: true}
	}
	base := v1alpha1.AnomalyRuleForProvider{ProjectID: "prj_1", Metric: "queries_per_hour"}

	anomaly := []struct {
		name   string
		server func(*pgbeam.AnomalyRule)
		want   bool
	}{
		{"nothing set on either side", func(*pgbeam.AnomalyRule) {}, true},
		{"credential scope held by the server", func(a *pgbeam.AnomalyRule) { a.CredentialId = &cred }, false},
		{"sigma threshold held by the server", func(a *pgbeam.AnomalyRule) { a.SigmaThreshold = ptr(4.5) }, false},
		{"floor held by the server", func(a *pgbeam.AnomalyRule) { a.Floor = ptr(100.0) }, false},
		{"rule disabled on the server", func(a *pgbeam.AnomalyRule) { a.Enabled = false }, false},
	}
	for _, tc := range anomaly {
		t.Run("AnomalyRule/"+tc.name, func(t *testing.T) {
			t.Parallel()
			a := rule()
			tc.server(a)
			if got := isAnomalyRuleUpToDate(base, a); got != tc.want {
				t.Errorf("isAnomalyRuleUpToDate = %v, want %v", got, tc.want)
			}
		})
	}

	readWrite := pgbeam.PolicyProfileAccessModeReadWrite
	profile := v1alpha1.PolicyProfileForProvider{ProjectID: "prj_1", Name: "agents"}
	policy := []struct {
		name   string
		server pgbeam.PolicyProfile
		want   bool
	}{
		{"defaults on the server", pgbeam.PolicyProfile{Name: "agents", AccessMode: pgbeam.PolicyProfileAccessModeReadOnly}, true},
		{"write access held by the server", pgbeam.PolicyProfile{Name: "agents", AccessMode: readWrite}, false},
		{"allowlist held by the server", pgbeam.PolicyProfile{Name: "agents", AccessMode: pgbeam.PolicyProfileAccessModeReadOnly, TableAllowlist: []string{"orders"}}, false},
		{"row cap held by the server", pgbeam.PolicyProfile{Name: "agents", AccessMode: pgbeam.PolicyProfileAccessModeReadOnly, MaxRows: ptr(10)}, false},
	}
	for _, tc := range policy {
		t.Run("PolicyProfile/"+tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isPolicyProfileUpToDate(profile, &tc.server); got != tc.want {
				t.Errorf("isPolicyProfileUpToDate = %v, want %v", got, tc.want)
			}
		})
	}

	limit := v1alpha1.SpendLimitForProvider{}
	spend := []struct {
		name   string
		server pgbeam.OrganizationPlan
		want   bool
	}{
		{"no limit on the server", pgbeam.OrganizationPlan{}, true},
		{"limit held by the server", pgbeam.OrganizationPlan{SpendLimit: ptr(50.0)}, false},
		// Zero is a limit (no overage at all), not the absence of one.
		{"zero limit held by the server", pgbeam.OrganizationPlan{SpendLimit: ptr(0.0)}, false},
	}
	for _, tc := range spend {
		t.Run("SpendLimit/"+tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isSpendLimitUpToDate(limit, &tc.server); got != tc.want {
				t.Errorf("isSpendLimitUpToDate = %v, want %v", got, tc.want)
			}
		})
	}

	cacheRule := v1alpha1.CacheRuleForProvider{CacheEnabled: true}
	cache := []struct {
		name   string
		server pgbeam.CacheRuleEntry
		want   bool
	}{
		{"project defaults on the server", pgbeam.CacheRuleEntry{CacheEnabled: true}, true},
		{"TTL held by the server", pgbeam.CacheRuleEntry{CacheEnabled: true, CacheTtlSeconds: ptr(600)}, false},
		{"SWR held by the server", pgbeam.CacheRuleEntry{CacheEnabled: true, CacheSwrSeconds: ptr(0)}, false},
	}
	for _, tc := range cache {
		t.Run("CacheRule/"+tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isCacheRuleUpToDate(cacheRule, &tc.server); got != tc.want {
				t.Errorf("isCacheRuleUpToDate = %v, want %v", got, tc.want)
			}
		})
	}

	gate := v1alpha1.ApprovalRuleForProvider{ProjectID: "prj_1", Name: "deletes"}
	rows := int32(1000)
	schema := "billing"
	approval := []struct {
		name   string
		server pgbeam.ApprovalRule
		want   bool
	}{
		{"matches everything on the server", pgbeam.ApprovalRule{Name: "deletes", Enabled: true}, true},
		{"row threshold held by the server", pgbeam.ApprovalRule{Name: "deletes", Enabled: true, MinAffectedRows: &rows}, false},
		{"schema scope held by the server", pgbeam.ApprovalRule{Name: "deletes", Enabled: true, SchemaName: &schema}, false},
		{"statement kinds held by the server", pgbeam.ApprovalRule{Name: "deletes", Enabled: true, StatementKinds: []pgbeam.ApprovalRuleStatementKind{"delete"}}, false},
		{"rule disabled on the server", pgbeam.ApprovalRule{Name: "deletes"}, false},
	}
	for _, tc := range approval {
		t.Run("ApprovalRule/"+tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isApprovalRuleUpToDate(gate, &tc.server); got != tc.want {
				t.Errorf("isApprovalRuleUpToDate = %v, want %v", got, tc.want)
			}
		})
	}
}
