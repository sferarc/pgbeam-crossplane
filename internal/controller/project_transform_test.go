package controller

import (
	"testing"
	"time"

	"github.com/pgbeam/provider-pgbeam/apis/v1alpha1"
	pgbeam "go.pgbeam.com/sdk"
)

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }
func intp(i int) *int       { return &i }
func i32p(i int32) *int32   { return &i }

func TestProjectObservation(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	p := &pgbeam.Project{
		Id:                "prj_1",
		ProxyHost:         strp("h.proxy.pgbeam.app"),
		QueriesPerSecond:  i32p(10),
		BurstSize:         i32p(20),
		MaxConnections:    i32p(30),
		DatabaseCount:     intp(2),
		ActiveConnections: intp(5),
		CreatedAt:         created,
		UpdatedAt:         updated,
	}

	obs := projectObservation(p)
	if obs.ID != "prj_1" {
		t.Errorf("ID = %q", obs.ID)
	}
	if obs.ProxyHost != "h.proxy.pgbeam.app" {
		t.Errorf("ProxyHost = %q", obs.ProxyHost)
	}
	// SDK exposes these as *int32; the observation flattens to int.
	if obs.QueriesPerSecond != 10 || obs.BurstSize != 20 || obs.MaxConnections != 30 {
		t.Errorf("rate limits = %d/%d/%d, want 10/20/30", obs.QueriesPerSecond, obs.BurstSize, obs.MaxConnections)
	}
	if obs.DatabaseCount != 2 || obs.ActiveConnections != 5 {
		t.Errorf("counts = %d/%d, want 2/5", obs.DatabaseCount, obs.ActiveConnections)
	}
	if obs.CreatedAt != created.Format(time.RFC3339) {
		t.Errorf("CreatedAt = %q", obs.CreatedAt)
	}
	if obs.UpdatedAt != updated.Format(time.RFC3339) {
		t.Errorf("UpdatedAt = %q", obs.UpdatedAt)
	}
}

func TestProjectObservation_NilOptionals(t *testing.T) {
	t.Parallel()

	p := &pgbeam.Project{Id: "prj_2", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	obs := projectObservation(p)
	if obs.ProxyHost != "" {
		t.Errorf("ProxyHost = %q, want empty", obs.ProxyHost)
	}
	if obs.QueriesPerSecond != 0 || obs.BurstSize != 0 || obs.MaxConnections != 0 {
		t.Error("nil rate limits should map to 0")
	}
}

func baseProject() *pgbeam.Project {
	return &pgbeam.Project{
		Id:     "prj_1",
		Name:   "same",
		Status: pgbeam.ProjectStatusActive,
	}
}

func TestIsProjectUpToDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fp   v1alpha1.ProjectForProvider
		p    func() *pgbeam.Project
		want bool
	}{
		{
			name: "identical name is up to date",
			fp:   v1alpha1.ProjectForProvider{Name: "same"},
			p:    baseProject,
			want: true,
		},
		{
			name: "name drift",
			fp:   v1alpha1.ProjectForProvider{Name: "different"},
			p:    baseProject,
			want: false,
		},
		{
			name: "description drift",
			fp:   v1alpha1.ProjectForProvider{Name: "same", Description: strp("new")},
			p:    baseProject,
			want: false,
		},
		{
			name: "description matches",
			fp:   v1alpha1.ProjectForProvider{Name: "same", Description: strp("desc")},
			p: func() *pgbeam.Project {
				p := baseProject()
				p.Description = strp("desc")
				return p
			},
			want: true,
		},
		{
			name: "tags length drift",
			fp:   v1alpha1.ProjectForProvider{Name: "same", Tags: []string{"a", "b"}},
			p: func() *pgbeam.Project {
				p := baseProject()
				p.Tags = &[]string{"a"}
				return p
			},
			want: false,
		},
		{
			name: "tags value drift",
			fp:   v1alpha1.ProjectForProvider{Name: "same", Tags: []string{"a"}},
			p: func() *pgbeam.Project {
				p := baseProject()
				p.Tags = &[]string{"b"}
				return p
			},
			want: false,
		},
		{
			name: "tags match",
			fp:   v1alpha1.ProjectForProvider{Name: "same", Tags: []string{"a", "b"}},
			p: func() *pgbeam.Project {
				p := baseProject()
				p.Tags = &[]string{"a", "b"}
				return p
			},
			want: true,
		},
		{
			name: "allowed_cidrs cidr drift",
			fp: v1alpha1.ProjectForProvider{
				Name:         "same",
				AllowedCidrs: []v1alpha1.CidrEntryParameters{{Cidr: "10.0.0.0/8"}},
			},
			p: func() *pgbeam.Project {
				p := baseProject()
				p.AllowedCidrs = &[]pgbeam.CidrEntry{{Cidr: "192.168.0.0/16"}}
				return p
			},
			want: false,
		},
		{
			name: "allowed_cidrs label drift",
			fp: v1alpha1.ProjectForProvider{
				Name:         "same",
				AllowedCidrs: []v1alpha1.CidrEntryParameters{{Cidr: "10.0.0.0/8", Label: strp("office")}},
			},
			p: func() *pgbeam.Project {
				p := baseProject()
				p.AllowedCidrs = &[]pgbeam.CidrEntry{{Cidr: "10.0.0.0/8", Label: strp("home")}}
				return p
			},
			want: false,
		},
		{
			name: "allowed_cidrs match without labels",
			fp: v1alpha1.ProjectForProvider{
				Name:         "same",
				AllowedCidrs: []v1alpha1.CidrEntryParameters{{Cidr: "10.0.0.0/8"}},
			},
			p: func() *pgbeam.Project {
				p := baseProject()
				p.AllowedCidrs = &[]pgbeam.CidrEntry{{Cidr: "10.0.0.0/8"}}
				return p
			},
			want: true,
		},
		{
			name: "agents_disabled drift",
			fp:   v1alpha1.ProjectForProvider{Name: "same", AgentsDisabled: boolp(true)},
			p:    baseProject,
			want: false,
		},
		{
			name: "default_policy_profile drift",
			fp:   v1alpha1.ProjectForProvider{Name: "same", DefaultPolicyProfileID: strp("pol_1")},
			p:    baseProject,
			want: false,
		},
		{
			name: "status drift",
			fp:   v1alpha1.ProjectForProvider{Name: "same", Status: "suspended"},
			p:    baseProject,
			want: false,
		},
		{
			name: "status match",
			fp:   v1alpha1.ProjectForProvider{Name: "same", Status: "active"},
			p:    baseProject,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isProjectUpToDate(tt.fp, tt.p()); got != tt.want {
				t.Errorf("isProjectUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsProjectUpToDate_LabelPointerComparison verifies that CIDR labels are
// compared by value, not pointer identity. Previously isProjectUpToDate compared
// fp.AllowedCidrs[i].Label != p.AllowedCidrs[i].Label (both *string), so two
// distinct pointers to the same string compared unequal, a project whose CIDR
// labels matched the desired state was reported as NOT up to date on every
// reconcile, causing a perpetual update loop. The generator now dereferences
// and compares label values with nil handling.
func TestIsProjectUpToDate_LabelPointerComparison(t *testing.T) {
	t.Parallel()

	t.Run("matching labels are up to date", func(t *testing.T) {
		t.Parallel()
		fp := v1alpha1.ProjectForProvider{
			Name:         "same",
			AllowedCidrs: []v1alpha1.CidrEntryParameters{{Cidr: "10.0.0.0/8", Label: strp("office")}},
		}
		p := baseProject()
		p.AllowedCidrs = &[]pgbeam.CidrEntry{{Cidr: "10.0.0.0/8", Label: strp("office")}}

		if !isProjectUpToDate(fp, p) {
			t.Error("matching labels should be up to date")
		}
	})

	t.Run("differing label values drift", func(t *testing.T) {
		t.Parallel()
		fp := v1alpha1.ProjectForProvider{
			Name:         "same",
			AllowedCidrs: []v1alpha1.CidrEntryParameters{{Cidr: "10.0.0.0/8", Label: strp("office")}},
		}
		p := baseProject()
		p.AllowedCidrs = &[]pgbeam.CidrEntry{{Cidr: "10.0.0.0/8", Label: strp("home")}}

		if isProjectUpToDate(fp, p) {
			t.Error("differing label values should drift")
		}
	})

	t.Run("nil vs non-nil label drifts", func(t *testing.T) {
		t.Parallel()
		fp := v1alpha1.ProjectForProvider{
			Name:         "same",
			AllowedCidrs: []v1alpha1.CidrEntryParameters{{Cidr: "10.0.0.0/8", Label: strp("office")}},
		}
		p := baseProject()
		p.AllowedCidrs = &[]pgbeam.CidrEntry{{Cidr: "10.0.0.0/8", Label: nil}}

		if isProjectUpToDate(fp, p) {
			t.Error("nil vs non-nil label should drift")
		}
	})
}
