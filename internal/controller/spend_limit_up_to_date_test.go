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

func TestSpendLimitIdenticalIsUpToDate(t *testing.T) {
	t.Parallel()
	fp := v1alpha1.SpendLimitForProvider{OrgID: "org_1", SpendLimit: ptr(250.0)}
	if !isSpendLimitUpToDate(fp, &pgbeam.OrganizationPlan{OrgId: "org_1", SpendLimit: ptr(250.0)}) {
		t.Fatal("identical spec and API object reported as drift")
	}
}

func TestSpendLimitFieldDrift(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*pgbeam.OrganizationPlan){
		"SpendLimit": func(o *pgbeam.OrganizationPlan) { o.SpendLimit = ptr(10000.0) },
	}
	requireCases(t, reflect.TypeFor[v1alpha1.SpendLimitForProvider](), cases, "OrgID")

	extra := map[string]func(*pgbeam.OrganizationPlan){
		"limit removed in the console": func(o *pgbeam.OrganizationPlan) { o.SpendLimit = nil },
		"limit lowered by a cent":      func(o *pgbeam.OrganizationPlan) { o.SpendLimit = ptr(249.99) },
	}
	for name, edit := range extra {
		cases[name] = edit
	}

	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			o := &pgbeam.OrganizationPlan{OrgId: "org_1", SpendLimit: ptr(250.0)}
			edit(o)
			if isSpendLimitUpToDate(v1alpha1.SpendLimitForProvider{OrgID: "org_1", SpendLimit: ptr(250.0)}, o) {
				t.Errorf("%s not detected", name)
			}
		})
	}
}

// Omitting spendLimit declares no limit, which is the API default.
func TestSpendLimitOmittedComparesAgainstNoLimit(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeFor[v1alpha1.SpendLimitForProvider]()
	if got := optionalFields(typ); len(got) != 1 || got[0] != "SpendLimit" {
		t.Fatalf("optional fields = %v, want [SpendLimit]", got)
	}

	minimal := v1alpha1.SpendLimitForProvider{OrgID: "org_1"}
	if !isSpendLimitUpToDate(minimal, &pgbeam.OrganizationPlan{OrgId: "org_1"}) {
		t.Error("no limit on either side reported as drift")
	}
	if isSpendLimitUpToDate(minimal, &pgbeam.OrganizationPlan{OrgId: "org_1", SpendLimit: ptr(100.0)}) {
		t.Error("limit set in the console not detected")
	}
}

func TestSpendLimitUpdateSendsDesiredLimit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want *float64
	}{
		{"set", ptr(250.0)},
		{"cleared", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			api := newFakeAPI(t, &pgbeam.OrganizationPlan{OrgId: "org_1", SpendLimit: ptr(9.99)})
			e := &spendLimitExternal{client: api.client()}
			fp := v1alpha1.SpendLimitForProvider{OrgID: "org_1", SpendLimit: tc.want}
			cr := &v1alpha1.SpendLimit{Spec: v1alpha1.SpendLimitSpec{ForProvider: fp}}
			meta.SetExternalName(cr, "org_1")

			if _, err := e.Update(context.Background(), cr); err != nil {
				t.Fatalf("Update: %v", err)
			}

			// A PUT without spend_limit is how the API is told to remove it.
			call, _ := api.onlyWrite(t, "PUT")
			var applied pgbeam.OrganizationPlan
			if err := json.Unmarshal(call.body, &applied); err != nil {
				t.Fatal(err)
			}
			if !isSpendLimitUpToDate(fp, &applied) {
				t.Errorf("PUT body %s does not carry the desired state", call.body)
			}
		})
	}
}
