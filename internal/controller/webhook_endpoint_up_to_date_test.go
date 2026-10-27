package controller

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/sferarc/pgbeam-crossplane/apis/v1alpha1"
	pgbeam "go.pgbeam.com/sdk"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func desiredWebhookEndpoint() v1alpha1.WebhookEndpointForProvider {
	return v1alpha1.WebhookEndpointForProvider{
		ProjectID:       "prj_1",
		URL:             "https://hooks.example.com/pgbeam",
		Format:          "splunk_hec",
		EventTypes:      []string{"anomaly_alert", "approval_requested"},
		Enabled:         ptr(false),
		Description:     ptr("siem"),
		SecretSecretRef: secretRef("crossplane-system", "hook", "secret"),
	}
}

func webhookEndpointMatching() *pgbeam.WebhookEndpoint {
	return &pgbeam.WebhookEndpoint{
		Id:          "whk_1",
		ProjectId:   "prj_1",
		Url:         "https://hooks.example.com/pgbeam",
		Format:      "splunk_hec",
		EventTypes:  &[]pgbeam.WebhookEventType{pgbeam.WebhookEventTypeAnomalyAlert, pgbeam.WebhookEventTypeApprovalRequested},
		Enabled:     false,
		Description: ptr("siem"),
	}
}

// webhookEndpointDefaults is what the API returns for an endpoint created with
// only a URL and secret.
func webhookEndpointDefaults() *pgbeam.WebhookEndpoint {
	return &pgbeam.WebhookEndpoint{
		Id:         "whk_1",
		ProjectId:  "prj_1",
		Url:        "https://hooks.example.com/pgbeam",
		Format:     "json",
		EventTypes: &[]pgbeam.WebhookEventType{},
		Enabled:    true,
	}
}

func TestWebhookEndpointIdenticalIsUpToDate(t *testing.T) {
	t.Parallel()
	if !isWebhookEndpointUpToDate(desiredWebhookEndpoint(), webhookEndpointMatching()) {
		t.Fatal("identical spec and API object reported as drift")
	}
}

func TestWebhookEndpointFieldDrift(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*pgbeam.WebhookEndpoint){
		"URL":         func(w *pgbeam.WebhookEndpoint) { w.Url = "https://attacker.example.com/" },
		"Format":      func(w *pgbeam.WebhookEndpoint) { w.Format = "json" },
		"EventTypes":  func(w *pgbeam.WebhookEndpoint) { w.EventTypes = nil },
		"Enabled":     func(w *pgbeam.WebhookEndpoint) { w.Enabled = true },
		"Description": func(w *pgbeam.WebhookEndpoint) { w.Description = ptr("renamed") },
	}
	// The secret is write-only: the API never returns it, so Update always
	// resends it instead of comparing.
	requireCases(t, reflect.TypeFor[v1alpha1.WebhookEndpointForProvider](), cases, "ProjectID", "SecretSecretRef")

	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := webhookEndpointMatching()
			edit(w)
			if isWebhookEndpointUpToDate(desiredWebhookEndpoint(), w) {
				t.Errorf("edit to %s not detected", name)
			}
		})
	}
}

func TestWebhookEndpointOmittedFieldsCompareAgainstDefault(t *testing.T) {
	t.Parallel()

	minimal := v1alpha1.WebhookEndpointForProvider{
		ProjectID:       "prj_1",
		URL:             "https://hooks.example.com/pgbeam",
		SecretSecretRef: secretRef("crossplane-system", "hook", "secret"),
	}
	if !isWebhookEndpointUpToDate(minimal, webhookEndpointDefaults()) {
		t.Fatal("minimal spec against API defaults reported as drift")
	}
	sparse := webhookEndpointDefaults()
	sparse.EventTypes = nil
	if !isWebhookEndpointUpToDate(minimal, sparse) {
		t.Fatal("minimal spec against a response without event types reported as drift")
	}

	offDefault := map[string]func(*pgbeam.WebhookEndpoint){
		"Format": func(w *pgbeam.WebhookEndpoint) { w.Format = "datadog" },
		"EventTypes": func(w *pgbeam.WebhookEndpoint) {
			w.EventTypes = &[]pgbeam.WebhookEventType{pgbeam.WebhookEventTypeAuditCheckpoint}
		},
		"Enabled":     func(w *pgbeam.WebhookEndpoint) { w.Enabled = false },
		"Description": func(w *pgbeam.WebhookEndpoint) { w.Description = ptr("set in the console") },
	}
	typ := reflect.TypeFor[v1alpha1.WebhookEndpointForProvider]()
	requireCases(t, typ, offDefault, "ProjectID", "URL", "SecretSecretRef")
	if got := len(optionalFields(typ)); got != len(offDefault) {
		t.Errorf("%d optional fields, %d cases", got, len(offDefault))
	}

	for name, edit := range offDefault {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := webhookEndpointDefaults()
			edit(w)
			if isWebhookEndpointUpToDate(minimal, w) {
				t.Errorf("omitted %s with a non-default server value not detected", name)
			}
		})
	}
}

func TestWebhookEndpointEventTypesDrift(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		types []pgbeam.WebhookEventType
	}{
		{"add", []pgbeam.WebhookEventType{"anomaly_alert", "approval_requested", "budget_exhausted"}},
		{"remove", []pgbeam.WebhookEventType{"anomaly_alert"}},
		{"reorder", []pgbeam.WebhookEventType{"approval_requested", "anomaly_alert"}},
		{"replace", []pgbeam.WebhookEventType{"anomaly_alert", "audit_checkpoint"}},
		{"cleared", []pgbeam.WebhookEventType{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := webhookEndpointMatching()
			w.EventTypes = &tc.types
			if isWebhookEndpointUpToDate(desiredWebhookEndpoint(), w) {
				t.Errorf("eventTypes %s not detected", tc.name)
			}
		})
	}
}

func TestWebhookEndpointUpdateSendsFullDesiredState(t *testing.T) {
	t.Parallel()

	api := newFakeAPI(t, webhookEndpointDefaults())
	kube := fake.NewClientBuilder().WithScheme(newTestScheme(t)).
		WithObjects(apiKeySecret("crossplane-system", "hook", "secret", "whsec_1")).
		Build()
	e := &webhookEndpointExternal{client: api.client(), kube: kube, connector: &connector{kube: kube, usage: noopTracker()}}
	cr := &v1alpha1.WebhookEndpoint{Spec: v1alpha1.WebhookEndpointSpec{ForProvider: desiredWebhookEndpoint()}}
	meta.SetExternalName(cr, "whk_1")

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update: %v", err)
	}

	call, keys := api.onlyWrite(t, "PUT")
	for _, k := range jsonFieldNames(reflect.TypeFor[pgbeam.WebhookEndpointInput]()) {
		if _, ok := keys[k]; !ok {
			t.Errorf("PUT body has no %q; the API would reset it to its default", k)
		}
	}
	if string(keys["secret"]) != `"whsec_1"` {
		t.Errorf("secret = %s, want the referenced secret value", keys["secret"])
	}
	var applied pgbeam.WebhookEndpoint
	if err := json.Unmarshal(call.body, &applied); err != nil {
		t.Fatal(err)
	}
	if !isWebhookEndpointUpToDate(desiredWebhookEndpoint(), &applied) {
		t.Errorf("PUT body %s does not carry the desired state", call.body)
	}
}
