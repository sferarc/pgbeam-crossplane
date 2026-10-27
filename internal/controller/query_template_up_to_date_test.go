package controller

import (
	"testing"

	"github.com/sferarc/pgbeam-crossplane/apis/v1alpha1"
	pgbeam "go.pgbeam.com/sdk"
)

// The API returns every param's nullable as a non-nil pointer, so a manifest
// that omits it must count as false, or Observe reports drift on every poll
// and each PUT pushes the template to the data planes again.
func TestQueryTemplateOmittedParamNullableIsUpToDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest *bool
		server   bool
		want     bool
	}{
		{"omitted matches server false", nil, false, true},
		{"explicit false matches server false", ptr(false), false, true},
		{"explicit true matches server true", ptr(true), true, true},
		{"omitted differs from server true", nil, true, false},
		{"explicit true differs from server false", ptr(true), false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fp := v1alpha1.QueryTemplateForProvider{
				ProjectID: "prj_1",
				Name:      "orders",
				SQL:       "SELECT 1",
				Params:    []v1alpha1.QueryTemplateParamParameters{{Name: "customer_id", Type: "bigint", Nullable: tc.manifest}},
			}
			q := &pgbeam.QueryTemplate{
				Name:    "orders",
				Sql:     "SELECT 1",
				Enabled: true,
				Params:  []pgbeam.QueryTemplateParam{{Name: "customer_id", Type: "bigint", Nullable: ptr(tc.server)}},
			}
			if got := isQueryTemplateUpToDate(fp, q); got != tc.want {
				t.Errorf("isQueryTemplateUpToDate = %v, want %v", got, tc.want)
			}
		})
	}
}
