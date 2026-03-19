package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ============================================================================
// ProviderConfig
// ============================================================================

// ProviderConfigSpec defines the desired state of the provider configuration.
type ProviderConfigSpec struct {
	// APIKeySecretRef is a reference to a Secret containing the PgBeam API key.
	// The key must be stored under the "apiKey" data field.
	// +kubebuilder:validation:Required
	APIKeySecretRef xpv1.SecretKeySelector `json:"apiKeySecretRef"`

	// BaseURL is the PgBeam API base URL. Defaults to https://api.pgbeam.com.
	// +optional
	// +kubebuilder:default="https://api.pgbeam.com"
	BaseURL string `json:"baseUrl,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=crossplane
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ProviderConfig configures how the provider connects to the PgBeam API.
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

// ProviderConfigUsage indicates a usage of a ProviderConfig.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=crossplane
type ProviderConfigUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	xpv1.ProviderConfigUsage `json:",inline"`
}

// +kubebuilder:object:root=true
type ProviderConfigUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfigUsage `json:"items"`
}

// ProviderConfigUsage interface methods.

func (p *ProviderConfigUsage) GetProviderConfigReference() xpv1.Reference {
	return p.ProviderConfigUsage.ProviderConfigReference
}

func (p *ProviderConfigUsage) SetProviderConfigReference(r xpv1.Reference) {
	p.ProviderConfigUsage.ProviderConfigReference = r
}

func (p *ProviderConfigUsage) SetResourceReference(r xpv1.TypedReference) {
	p.ProviderConfigUsage.ResourceReference = r
}

func (p *ProviderConfigUsage) GetResourceReference() xpv1.TypedReference {
	return p.ProviderConfigUsage.ResourceReference
}

// ProviderConfig interface methods.

func (p *ProviderConfig) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	// ProviderConfig doesn't have conditions in our simple implementation.
	return xpv1.Condition{}
}

func (p *ProviderConfig) SetConditions(_ ...xpv1.Condition) {}

func (p *ProviderConfig) GetUsers() int64 { return 0 }

func (p *ProviderConfig) SetUsers(_ int64) {}
