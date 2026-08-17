package controller

import (
	"context"
	"errors"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pgbeam/provider-pgbeam/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newTestScheme registers the provider and core K8s types on a scheme for the
// fake client.
func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add provider scheme: %v", err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	return s
}

// noopTracker satisfies resource.Tracker without side effects.
func noopTracker() resource.Tracker {
	return resource.TrackerFn(func(_ context.Context, _ resource.Managed) error { return nil })
}

func apiKeySecret(namespace, name, key, value string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Data:       map[string][]byte{key: []byte(value)},
	}
}

func secretRef(namespace, name, key string) xpv1.SecretKeySelector {
	return xpv1.SecretKeySelector{
		SecretReference: xpv1.SecretReference{Namespace: namespace, Name: name},
		Key:             key,
	}
}

func TestGetSecretValue_Success(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	kube := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(apiKeySecret("crossplane-system", "pgbeam-creds", "apiKey", "pgb_secret")).
		Build()

	c := &connector{kube: kube, usage: noopTracker()}
	val, err := c.getSecretValue(context.Background(), secretRef("crossplane-system", "pgbeam-creds", "apiKey"))
	if err != nil {
		t.Fatalf("getSecretValue: %v", err)
	}
	if val != "pgb_secret" {
		t.Errorf("value = %q, want pgb_secret", val)
	}
}

func TestGetSecretValue_MissingSecret(t *testing.T) {
	t.Parallel()

	kube := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	c := &connector{kube: kube, usage: noopTracker()}

	_, err := c.getSecretValue(context.Background(), secretRef("crossplane-system", "absent", "apiKey"))
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestGetSecretValue_MissingKey(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	kube := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(apiKeySecret("crossplane-system", "pgbeam-creds", "apiKey", "pgb_secret")).
		Build()

	c := &connector{kube: kube, usage: noopTracker()}
	_, err := c.getSecretValue(context.Background(), secretRef("crossplane-system", "pgbeam-creds", "wrongKey"))
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestGetClient_Success(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	pc := &v1alpha1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1alpha1.ProviderConfigSpec{
			APIKeySecretRef: secretRef("crossplane-system", "pgbeam-creds", "apiKey"),
			BaseURL:         "https://api.example.com",
		},
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(pc, apiKeySecret("crossplane-system", "pgbeam-creds", "apiKey", "pgb_secret")).
		Build()

	c := &connector{kube: kube, usage: noopTracker()}

	mg := &v1alpha1.Project{}
	mg.SetProviderConfigReference(&xpv1.Reference{Name: "default"})

	client, err := c.getClient(context.Background(), mg)
	if err != nil {
		t.Fatalf("getClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Projects == nil {
		t.Error("expected Projects service to be wired")
	}
}

func TestGetClient_NoProviderConfigRef(t *testing.T) {
	t.Parallel()

	kube := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	c := &connector{kube: kube, usage: noopTracker()}

	mg := &v1alpha1.Project{} // no provider config reference set
	_, err := c.getClient(context.Background(), mg)
	if err == nil {
		t.Fatal("expected error when no providerConfigRef is set")
	}
}

func TestGetClient_MissingProviderConfig(t *testing.T) {
	t.Parallel()

	kube := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	c := &connector{kube: kube, usage: noopTracker()}

	mg := &v1alpha1.Project{}
	mg.SetProviderConfigReference(&xpv1.Reference{Name: "absent"})
	_, err := c.getClient(context.Background(), mg)
	if err == nil {
		t.Fatal("expected error when ProviderConfig does not exist")
	}
}

func TestGetClient_TrackerError(t *testing.T) {
	t.Parallel()

	kube := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	sentinel := errors.New("track boom")
	c := &connector{
		kube:  kube,
		usage: resource.TrackerFn(func(_ context.Context, _ resource.Managed) error { return sentinel }),
	}

	mg := &v1alpha1.Project{}
	mg.SetProviderConfigReference(&xpv1.Reference{Name: "default"})
	_, err := c.getClient(context.Background(), mg)
	if err == nil {
		t.Fatal("expected error when usage tracking fails")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want wrapped %v", err, sentinel)
	}
}
