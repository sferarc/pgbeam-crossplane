// Package v1alpha1 contains the v1alpha1 API group for PgBeam managed resources.
//
// +kubebuilder:object:generate=true
// +groupName=pgbeam.io
// +versionName=v1alpha1
package v1alpha1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package type metadata.
const (
	Group   = "pgbeam.io"
	Version = "v1alpha1"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionResource scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(
		&Project{}, &ProjectList{},
		&Database{}, &DatabaseList{},
		&Replica{}, &ReplicaList{},
		&CustomDomain{}, &CustomDomainList{},
		&CacheRule{}, &CacheRuleList{},
		&SpendLimit{}, &SpendLimitList{},
		&ProviderConfig{}, &ProviderConfigList{},
		&ProviderConfigUsage{}, &ProviderConfigUsageList{},
	)
}

// Project type metadata.
var (
	ProjectKind             = reflect.TypeOf(Project{}).Name()
	ProjectGroupKind        = schema.GroupKind{Group: Group, Kind: ProjectKind}.String()
	ProjectKindAPIVersion   = ProjectKind + "." + SchemeGroupVersion.String()
	ProjectGroupVersionKind = SchemeGroupVersion.WithKind(ProjectKind)
)

// Database type metadata.
var (
	DatabaseKind             = reflect.TypeOf(Database{}).Name()
	DatabaseGroupKind        = schema.GroupKind{Group: Group, Kind: DatabaseKind}.String()
	DatabaseKindAPIVersion   = DatabaseKind + "." + SchemeGroupVersion.String()
	DatabaseGroupVersionKind = SchemeGroupVersion.WithKind(DatabaseKind)
)

// Replica type metadata.
var (
	ReplicaKind             = reflect.TypeOf(Replica{}).Name()
	ReplicaGroupKind        = schema.GroupKind{Group: Group, Kind: ReplicaKind}.String()
	ReplicaKindAPIVersion   = ReplicaKind + "." + SchemeGroupVersion.String()
	ReplicaGroupVersionKind = SchemeGroupVersion.WithKind(ReplicaKind)
)

// CustomDomain type metadata.
var (
	CustomDomainKind             = reflect.TypeOf(CustomDomain{}).Name()
	CustomDomainGroupKind        = schema.GroupKind{Group: Group, Kind: CustomDomainKind}.String()
	CustomDomainKindAPIVersion   = CustomDomainKind + "." + SchemeGroupVersion.String()
	CustomDomainGroupVersionKind = SchemeGroupVersion.WithKind(CustomDomainKind)
)

// CacheRule type metadata.
var (
	CacheRuleKind             = reflect.TypeOf(CacheRule{}).Name()
	CacheRuleGroupKind        = schema.GroupKind{Group: Group, Kind: CacheRuleKind}.String()
	CacheRuleKindAPIVersion   = CacheRuleKind + "." + SchemeGroupVersion.String()
	CacheRuleGroupVersionKind = SchemeGroupVersion.WithKind(CacheRuleKind)
)

// SpendLimit type metadata.
var (
	SpendLimitKind             = reflect.TypeOf(SpendLimit{}).Name()
	SpendLimitGroupKind        = schema.GroupKind{Group: Group, Kind: SpendLimitKind}.String()
	SpendLimitKindAPIVersion   = SpendLimitKind + "." + SchemeGroupVersion.String()
	SpendLimitGroupVersionKind = SchemeGroupVersion.WithKind(SpendLimitKind)
)
