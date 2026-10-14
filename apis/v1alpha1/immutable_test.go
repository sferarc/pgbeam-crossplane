package v1alpha1_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

// immutableFields maps each managed resource kind to the json names of the
// forProvider fields tagged +immutable in types.gen.go.
func immutableFields(t *testing.T) map[string][]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "types.gen.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || !strings.HasSuffix(ts.Name.Name, "ForProvider") {
			return true
		}
		kind := strings.TrimSuffix(ts.Name.Name, "ForProvider")
		out[kind] = []string{}
		for _, f := range ts.Type.(*ast.StructType).Fields.List {
			if f.Doc == nil || !strings.Contains(f.Doc.Text(), "+immutable") {
				continue
			}
			tag := reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("json")
			out[kind] = append(out[kind], strings.Split(tag, ",")[0])
		}
		return false
	})
	if len(out) == 0 {
		t.Fatal("no ForProvider structs found in types.gen.go")
	}
	return out
}

func loadCRDs(t *testing.T) map[string]*apiextensionsv1.CustomResourceDefinition {
	t.Helper()
	paths, err := filepath.Glob("../../package/crds/*.yaml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no CRDs found: %v", err)
	}
	out := map[string]*apiextensionsv1.CustomResourceDefinition{}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := yaml.Unmarshal(raw, crd); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out[crd.Spec.Names.Kind] = crd
	}
	return out
}

func forProviderSchema(t *testing.T, crd *apiextensionsv1.CustomResourceDefinition) *apiextensionsv1.JSONSchemaProps {
	t.Helper()
	fp, ok := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["forProvider"]
	if !ok {
		t.Fatalf("%s: no spec.forProvider", crd.Name)
	}
	return &fp
}

func TestImmutableFieldsHaveTransitionRules(t *testing.T) {
	crds := loadCRDs(t)
	for kind, fields := range immutableFields(t) {
		crd, ok := crds[kind]
		if !ok {
			t.Errorf("%s: no CRD", kind)
			continue
		}
		rules := forProviderSchema(t, crd).XValidations
		if len(rules) != len(fields) {
			t.Errorf("%s: %d immutable fields but %d forProvider rules", kind, len(fields), len(rules))
		}
		for _, name := range fields {
			want := "self." + name + " == oldSelf." + name
			found := false
			for _, r := range rules {
				if strings.Contains(r.Rule, want) && strings.Contains(r.Message, name+" is immutable") {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no transition rule for immutable field %s", kind, name)
			}
		}
	}
}

// TestCRDsPassAPIServerValidation runs the checks the API server applies when
// the CRD is installed, including CEL compilation and cost estimation.
func TestCRDsPassAPIServerValidation(t *testing.T) {
	for kind, crd := range loadCRDs(t) {
		// The API server fills storedVersions on create; validation requires it.
		crd.Status.StoredVersions = []string{crd.Spec.Versions[0].Name}
		internal := &apiextensions.CustomResourceDefinition{}
		if err := apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(crd, internal, nil); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if errs := validation.ValidateCustomResourceDefinition(context.Background(), internal); len(errs) > 0 {
			t.Errorf("%s: %v", kind, errs.ToAggregate())
		}
	}
}

func sampleValue(s apiextensionsv1.JSONSchemaProps, variant int) any {
	switch s.Type {
	case "integer", "number":
		return int64(1 + variant)
	case "boolean":
		return variant == 0
	case "object":
		obj := map[string]any{}
		for name, p := range s.Properties {
			if p.Type == "string" {
				obj[name] = sampleValue(p, variant)
			}
		}
		return obj
	}
	if s.Format == "date-time" {
		return []string{"2030-01-01T00:00:00Z", "2031-01-01T00:00:00Z"}[variant]
	}
	return []string{"a", "b"}[variant]
}

func TestImmutableFieldChangesAreRejected(t *testing.T) {
	crds := loadCRDs(t)
	for kind, fields := range immutableFields(t) {
		if len(fields) == 0 {
			continue
		}
		fpProps := forProviderSchema(t, crds[kind])
		internal := &apiextensions.JSONSchemaProps{}
		if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(fpProps, internal, nil); err != nil {
			t.Fatal(err)
		}
		structural, err := schema.NewStructural(internal)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		validator := cel.NewValidator(structural, false, celconfig.PerCallLimit)
		required := map[string]bool{}
		for _, r := range fpProps.Required {
			required[r] = true
		}
		base := map[string]any{}
		for _, name := range fields {
			if required[name] {
				base[name] = sampleValue(fpProps.Properties[name], 0)
			}
		}
		validate := func(newObj, oldObj map[string]any) field.ErrorList {
			errs, _ := validator.Validate(context.Background(), field.NewPath("forProvider"), structural, newObj, oldObj, math.MaxInt64)
			return errs
		}
		if errs := validate(base, base); len(errs) > 0 {
			t.Errorf("%s: unchanged forProvider rejected: %v", kind, errs.ToAggregate())
		}
		for _, name := range fields {
			changed := clone(base)
			changed[name] = sampleValue(fpProps.Properties[name], 1)
			withValue := clone(base)
			withValue[name] = sampleValue(fpProps.Properties[name], 0)
			cases := map[string][2]map[string]any{"change": {withValue, changed}}
			if !required[name] {
				cases["set"] = [2]map[string]any{base, withValue}
				cases["unset"] = [2]map[string]any{withValue, base}
			}
			for label, c := range cases {
				if errs := validate(c[1], c[0]); len(errs) == 0 {
					t.Errorf("%s: %s of immutable field %s was accepted", kind, label, name)
				}
			}
		}
	}
}

func clone(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
